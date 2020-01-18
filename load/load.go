// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package load delives final carton to user
package load

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"skygo/carton"
	"skygo/runbook"
	"skygo/runbook/xsync"
	"skygo/utils"
	"skygo/utils/log"
)

// Load represent state of load
type Load struct {
	loaders int // the number of loaders
	pool    *xsync.Pool

	// vars       map[string]string //global key-value
	runbook.KV //global key-value

	arg  []*runbook.Arg
	bufs []*bytes.Buffer

	// loadError is allowed to set only once
	err  loadError
	once sync.Once

	ctx    context.Context
	cancel func()
	exit   func() // clean up function

	refcount
	loaded [2]sync.Map

	loadCh chan *cartonMeta
}

type cartonMeta struct {
	carton   carton.Builder
	isNative bool
}

type pool struct {
	arg *runbook.Arg
	buf *bytes.Buffer
}

type loadError struct {
	err    error
	carton string        // error occurs on which carton
	buf    *bytes.Buffer // hold error log message
}

func (l *loadError) Error() string {

	var str strings.Builder

	fmt.Fprintf(&str, "\n\x1b[0;34m❯❯❯❯❯❯❯❯❯❯❯❯  %s\x1b[0m\n%s", l.carton, l.err) // blue(34)
	if l.buf != nil && l.buf.Len() > 0 {
		str.WriteString(fmt.Sprintf("\n\n\x1b[0;31m%s \x1b[0m", "Error log: ↡\n")) // red(31)
		str.Write(l.buf.Bytes())
	}
	return str.String()
}

// NewLoad create load to build carton
// loaders represent how many loader work. if its value is 0, it will use default value
// BUG: it has no chance to modify BUILDIR etc after NewLoad
func NewLoad(ctx context.Context, name string, loaders int) (*Load, int) {

	if loaders == 0 {
		loaders = 2 * runtime.NumCPU()
	}
	log.Trace("MaxLoaders is set to %d\n", loaders)

	load := Load{
		arg:     make([]*runbook.Arg, loaders),
		bufs:    make([]*bytes.Buffer, loaders),
		loaders: loaders,
		loadCh:  make(chan *cartonMeta),
	}

	loadDefaultCfg(&load.KV)

	buildir := defaultVars[BUILDIR].(string)
	os.MkdirAll(buildir, 0755)
	lockfile := filepath.Join(buildir, name+".lockfile")

	if utils.IsExist(lockfile) {
		fmt.Printf("another instance %s is running", name)
		os.Exit(1)
	}

	if err := carton.BuildInventory(ctx); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if _, err := os.Create(lockfile); err != nil {
		fmt.Println("Failed to create lockfile", lockfile)
		os.Exit(1)
	}
	log.Trace("Create lock file %s", lockfile)

	load.pool = xsync.NewPool(loaders, func(i int) interface{} {
		x := pool{
			arg: new(runbook.Arg),
			buf: new(bytes.Buffer),
		}

		x.arg.SetUnderOutput(nil, x.buf)
		load.arg[i] = x.arg
		load.bufs[i] = x.buf
		return &x
	})

	ctx, cancel := context.WithCancel(ctx)
	load.ctx = ctx
	load.exit = func() {
		os.Remove(lockfile)
		log.Trace("Delete lock file %s", lockfile)
	}
	load.cancel = func() {
		cancel()
		close(load.loadCh)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Trace("loader: cancel context by signal %s", sig)
		load.cancel()
	}()

	os.MkdirAll(load.GetStr(DLDIR), 0755)
	os.MkdirAll(load.GetStr(IMAGEDIR), 0755)
	return &load, loaders
}

// SetOutput assign stdout & stderr for one load
func (l *Load) SetOutput(index int, stdout, stderr io.Writer) *Load {
	if index >= l.loaders {
		return nil
	}

	l.arg[index].SetUnderOutput(stdout,
		io.MultiWriter(stderr, l.bufs[index]))
	return l
}

func (l *Load) perform(c carton.Builder, target string,
	isNative bool) (err error) {

	y, err := l.pool.Get(l.ctx)
	if err != nil {
		return err
	}
	defer l.pool.Put(y)
	x := y.(*pool)

	l.setupArg(c, x.arg, isNative)

	timeout := x.arg.GetStr("TIMEOUT")
	timeOut, _ := strconv.Atoi(timeout)
	ctx, cancel := context.WithTimeout(l.ctx, time.Duration(timeOut)*time.Second)
	defer cancel()

	ctx = runbook.NewContext(ctx, x.arg)

	// reset buffer
	x.buf.Reset()

	os.MkdirAll(x.arg.GetStr("WORKDIR"), 0755) //WORKDIR
	os.MkdirAll(x.arg.GetStr("T"), 0755)       //temp dir
	os.MkdirAll(x.arg.GetStr("D"), 0755)
	os.MkdirAll(x.arg.GetStr("PKGD"), 0755)

	if err = c.Runbook().Play(ctx, target, l); err != nil {
		l.once.Do(func() {
			l.err = loadError{
				carton: x.arg.Owner,
				buf:    x.buf,
				err:    err,
			}
		})
		return &l.err
	}
	return nil
}

func (l *Load) setupArg(carton carton.Builder, arg *runbook.Arg,
	isNative bool) {

	arg.Owner = carton.Provider()
	arg.FilesPath = carton.FilesPath()
	arg.Private = carton
	arg.Kv = carton
	arg.KV.Init(arg.Owner)

	wd := WorkDir(carton, isNative)

	// export global key-value to each carton's context
	l.Range(func(k, v string) { arg.Set(k, v) })

	// key-value for each carton's context
	for k, v := range map[string]string{
		"WORKDIR": wd,

		"PN":   carton.Provider(), // PN: provider name
		"T":    filepath.Join(wd, "temp"),
		"D":    filepath.Join(wd, "image"),    // install destination directory
		"PKGD": filepath.Join(wd, "packages"), // points to directory for files to be packaged

		"TARGETARCH":   getTargetArch(carton, isNative),
		"TARGETOS":     getTargetOS(carton, isNative),
		"TARGETVENDOR": getTargetVendor(carton, isNative),
	} {
		arg.Set(k, v)
	}
	arg.Set("ISNATIVE", isNative)

	if dir := carton.SrcDir(wd); dir != "" {
		arg.Set("S", dir)
	}
}

func (l *Load) setupRunbook(c carton.Builder) {

	rb := c.Runbook()

	if s := rb.Stage(carton.PATCH); s != nil {
		s.AddTask(0, func(ctx context.Context, dir string) error {
			return patch(ctx, dir)
		})
	}

	if s := rb.Stage(carton.SYSROOT); s != nil {
		s.AddTask(0, func(ctx context.Context, dir string) error {
			return prepare_sysroot(ctx, dir)
		})
	}

	rb.NewTaskForce("cleanall", cleanall,
		"Remove all intermediate stuff")
	rb.NewTaskForce("printenv", printenv,
		"Show global and per carton context variables")
	rb.NewTaskForce("cleanstate", cleanstate,
		"Clean state cache of all stages, same as flag --force")

	addEventListener(rb)
}

func (l *Load) find(name string) (c carton.Builder, isVirtual bool,
	isNative bool, err error) {

	c, isVirtual, isNative, err = carton.Find(name)
	if err != nil {
		l.once.Do(func() {
			l.err = loadError{
				carton: name,
				err:    err,
			}
			err = &l.err
		})
		return
	}

	// don't setup Runbook on carton's link
	t := c
	if isVirtual {
		t, _, _, _ = carton.Find(c.Provider())
	}
	l.setupRunbook(t)
	return
}

func (l *Load) Find(name string) (c carton.Builder, isVirtual bool,
	isNative bool, err error) {
	l.exit()
	return l.find(name)
}

func index(isNative bool) int {

	index := 0
	if isNative {
		index = 1
	}
	return index
}

func (l *Load) Wait(runbook, stage string, isNative bool) <-chan struct{} {

	if _, ok := l.loaded[index(isNative)].Load(runbook); ok {
		done := make(chan struct{})
		close(done)
		return done
	}

	c, _, native, _ := l.find(runbook)

	// inherits isNative
	if native {
		isNative = true
	}

	l.refGet()
	l.loadCh <- &cartonMeta{
		carton:   c,
		isNative: isNative,
	}

	if stage == "" {
		stage = "package"
	}
	return c.Runbook().Stage(stage).Wait(isNative)
}

func (l *Load) run(c carton.Builder, isNative bool) {

	wait := func(deps []string) {
		for _, carton := range deps {

			<-l.Wait(carton, "", isNative)
		}
	}

	wait(c.BuildDepends())
	wait(c.Depends())

	if err := l.perform(c, "", isNative); err != nil {
		l.cancel()
		return
	}

	l.loaded[index(isNative)].LoadOrStore(c.Provider(), struct{}{})
	l.refPut(func() { close(l.loadCh) })
}

func (l *Load) start(c carton.Builder, target string, nodeps, isNative bool) {

	rb := c.Runbook()
	if rb.HasTaskForce(target) {
		nodeps = true
	}

	wait := func(deps []string) {
		for _, carton := range deps {

			<-l.Wait(carton, "", isNative)
		}
	}

	l.refGet()
	go func() {

		if !nodeps {
			wait(c.BuildDepends())
			wait(c.Depends())
		}

		if err := l.perform(c, target, isNative); err != nil {
			l.cancel()
			return
		}

		l.loaded[index(isNative)].LoadOrStore(c.Provider(), struct{}{})
		l.refPut(func() { close(l.loadCh) })
	}()
}

func (l *Load) Run(carton, target string, nodeps, force bool) error {

	defer l.exit()

	c, _, isNative, err := l.find(carton)
	if err != nil {
		return err
	}

	if force {
		t := tempDir(c, isNative)
		cleanstate1(c, target, t)
	}

	l.start(c, target, nodeps, isNative)

	// run & wait done
	for meta := range l.loadCh {
		go l.run(meta.carton, meta.isNative)
	}

	if l.err.err != nil {
		return &l.err
	}
	return nil
}

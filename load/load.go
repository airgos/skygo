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
	"skygo/config"
	"skygo/runbook"
	"skygo/runbook/xsync"
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
	cancel context.CancelFunc

	exit func() // clean up function

	loaded sync.Map // record cartons loaded
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
func NewLoad(ctx context.Context, name string, loaders int) (*Load, int) {

	buildir := config.GetVar(config.BUILDIR)
	os.MkdirAll(buildir, 0755)
	lockfile := filepath.Join(buildir, name+".lockfile")

	if _, err := os.Stat(lockfile); err == nil {
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

	if loaders == 0 {
		loaders = 2 * runtime.NumCPU()
	}
	log.Trace("MaxLoaders is set to %d\n", loaders)

	load := Load{
		arg:     make([]*runbook.Arg, loaders),
		bufs:    make([]*bytes.Buffer, loaders),
		loaders: loaders,
	}

	load.KV.Init2("loader", map[string]interface{}{
		"TIMEOUT": "1800", // unit is second, default is 30min

		"DLDIR":    config.GetVar(config.DLDIR),
		"TOPDIR":   config.GetVar(config.TOPDIR),
		"IMAGEDIR": config.GetVar(config.IMAGEDIR),
	})

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

	load.ctx, load.cancel = context.WithCancel(ctx)
	load.exit = func() {
		os.Remove(lockfile)
		log.Trace("Delete lock file %s", lockfile)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		if load.cancel != nil {
			log.Trace("loader: cancel context by signal %s", sig)
			load.cancel()
		}
	}()

	os.MkdirAll(config.GetVar(config.DLDIR), 0755)
	os.MkdirAll(config.GetVar(config.IMAGEDIR), 0755)
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

func (l *Load) perform(ctx context.Context, c carton.Builder, target string,
	nodeps bool, isNative bool) (err error) {

	y, err := l.pool.Get(l.ctx)
	if err != nil {
		return err
	}
	defer l.pool.Put(y)
	x := y.(*pool)

	l.setupArg(c, x.arg, isNative)
	if nodeps {
		x.arg.SetUnderOutput(os.Stdout, os.Stderr)
	}

	timeout, _ := x.arg.LookupVar("TIMEOUT")
	timeOut, _ := strconv.Atoi(timeout)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeOut)*time.Second)
	defer cancel()

	ctx = runbook.NewContext(ctx, x.arg)

	// reset buffer
	x.buf.Reset()

	os.MkdirAll(x.arg.GetVar("WORKDIR"), 0755) //WORKDIR
	os.MkdirAll(x.arg.GetVar("T"), 0755)       //temp dir
	os.MkdirAll(x.arg.GetVar("D"), 0755)
	os.MkdirAll(x.arg.GetVar("PKGD"), 0755)

	if nodeps && target != "" {
		err = c.Runbook().Play(ctx, target)
	} else {
		err = c.Runbook().Range(ctx, target)
	}

	if err != nil {
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

func (l *Load) run(ctx context.Context, name, target string, isNative bool) {
	var wg sync.WaitGroup

	b, _, native, err := l.find(name)
	if err != nil {
		return
	}

	// inherits isNative
	if native {
		isNative = true
	}

	scan := func(deps []string) {
		wg.Add(len(deps))
		for _, d := range deps {
			name := d
			if i := strings.LastIndex(d, "@"); i >= 0 {
				name = d[:i]
				if target == "" {
					target = d[i+1:]
				}
			}
			go func(ctx context.Context, name, target string) {

				select {
				default:
					l.run(ctx, name, target, isNative)
				case <-ctx.Done():
				}
				wg.Done()
			}(ctx, name, target)
		}
	}

	scan(b.BuildDepends())
	scan(b.Depends())
	wg.Wait()

	if err := l.perform(ctx, b, target, false, isNative); err != nil {
		l.cancel()
	}
}

// Run start loading
func (l *Load) Run(name, target string, nodeps, force bool) error {

	defer l.exit()

	c, _, isNative, err := l.find(name)
	if err != nil {
		return err
	}

	rb := c.Runbook()
	if rb.TaskSet().Has(target) {
		nodeps = true
	}

	if force {
		t := tempDir(c, isNative)
		cleanstate1(rb, target, t)
	}

	if nodeps {
		return l.perform(l.ctx, c, target, true, isNative)
	}

	l.run(l.ctx, name, target, false)
	if l.err.err != nil {
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
	l.Range(func(k, v string) { arg.SetKv(k, v) })

	// key-value for each carton's context
	for k, v := range map[string]string{
		"ISNATIVE": fmt.Sprintf("%v", isNative),
		"WORKDIR":  wd,

		"PN":   carton.Provider(), // PN: provider name
		"T":    filepath.Join(wd, "temp"),
		"D":    filepath.Join(wd, "image"),    // install destination directory
		"PKGD": filepath.Join(wd, "packages"), // points to directory for files to be packaged

		"TARGETARCH":   getTargetArch(carton, isNative),
		"TARGETOS":     getTargetOS(carton, isNative),
		"TARGETVENDOR": getTargetVendor(carton, isNative),
	} {
		arg.SetKv(k, v)
	}

	if dir := carton.SrcDir(wd); dir != "" {
		arg.SetKv("S", dir)
	}
}

func (l *Load) setupRunbook(c carton.Builder) {

	rb := c.Runbook()
	name := c.Provider()

	if _, ok := l.loaded.Load(name); ok {
		return // avoide configuration again
	}

	if s := rb.Stage(carton.PATCH); s != nil {
		s.AddTask(0, func(ctx context.Context, dir string) error {
			return patch(ctx, dir)
		})
	}

	tset := rb.TaskSet()
	tset.Add("cleanall", cleanall,
		"Remove all intermediate stuff")
	tset.Add("printenv", printenv,
		"Show global and per carton context variables")
	tset.Add("cleanstate", cleanstate,
		"Clean state cache of all stages")

	addEventListener(rb)
	l.loaded.LoadOrStore(name, true)
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

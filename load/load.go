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
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"merge/carton"
	"merge/config"
	"merge/runbook"
)

// Load represent state of load
type Load struct {
	works int
	ch    chan int

	arg  []*runbook.Arg
	bufs []*bytes.Buffer

	cancel context.CancelFunc

	// loadError is allowed to set only once
	loadError
	once sync.Once
}

type loadError struct {
	err    error
	carton string        // error occurs on which carton
	buf    *bytes.Buffer // hold error log message
}

// NewLoad create load to build carton
// num represent how many loader work. if its value is 0, it will use default value
func NewLoad(num int) *Load {

	if num == 0 {
		num = runtime.NumCPU()
	}
	load := Load{
		ch:    make(chan int, num),
		arg:   make([]*runbook.Arg, num),
		bufs:  make([]*bytes.Buffer, num),
		works: num,
	}
	for i := 0; i < num; i++ {
		arg := new(runbook.Arg)
		load.arg[i] = arg

		load.ch <- i

		buf := new(bytes.Buffer)
		arg.SetOutput(nil, buf)
		load.bufs[i] = buf
	}

	return &load
}

func (l *Load) get() int {
	return <-l.ch
}

func (l *Load) put(index int) {
	l.ch <- index
}

// SetOutput assign stdout & stderr for one load
func (l *Load) SetOutput(index int, stdout, stderr io.Writer) *Load {
	l.arg[index].SetOutput(stdout,
		io.MultiWriter(stderr, l.bufs[index]))
	return l
}

func (l *Load) perform(ctx context.Context, carton carton.Builder, target string,
	nodeps bool) (err error) {

	index := l.get()
	arg := l.arg[index]
	setupArg(carton, arg)
	if nodeps {
		arg.SetOutput(os.Stdout, os.Stderr)
	}
	ctx = runbook.NewContext(ctx, arg)

	// reset buffer
	l.bufs[index].Reset()

	if nodeps && target != "" {
		err = carton.Runbook().Play(ctx, target)
	} else {
		err = carton.Runbook().Perform(ctx, target)
	}
	l.put(index)

	if err != nil {
		l.once.Do(func() {
			l.carton = arg.Owner
			l.buf = l.bufs[index]
			l.err = err
		})
		return l
	}
	return nil
}

func (l *Load) run(ctx context.Context, name, target string) {
	var wg sync.WaitGroup

	b, _, err := carton.Find(name)
	if err != nil {
		l.err = err
		l.carton = name
		return
	}

	deps := b.BuildDepends()
	required := b.Depends()
	deps = append(deps, required...)

	wg.Add(len(deps))
	for _, d := range deps {
		name := d
		target := ""
		if i := strings.LastIndex(d, "@"); i >= 0 {
			name, target = d[:i], d[i+1:]
		}
		go func(ctx context.Context, name, target string) {

			select {
			default:
				l.run(ctx, name, target)
			case <-ctx.Done():
			}
			wg.Done()
		}(ctx, name, target)
	}
	wg.Wait()

	if err := l.perform(ctx, b, target, false); err != nil {
		l.cancel()
	}
}

// Run start loading
func (l *Load) Run(ctx context.Context, name, target string, nodeps bool) error {

	ctx, cancel := context.WithCancel(ctx)
	l.cancel = cancel

	os.MkdirAll(config.GetVar(config.BUILDIR), 0755)
	os.MkdirAll(config.GetVar(config.DLDIR), 0755)

	if nodeps {
		b, _, err := carton.Find(name)
		if err != nil {
			l.err = err
			l.carton = name
			return l
		}
		return l.perform(ctx, b, target, true)
	}

	l.run(ctx, name, target)
	if l.err != nil {
		return l
	}
	return nil
}

// Clean invokes carton's method Clean
func (l *Load) Clean(ctx context.Context, name string, force bool) error {

	c, _, err := carton.Find(name)
	if err != nil {
		return err
	}

	if force {
		os.RemoveAll(WorkDir(c, false))
		return nil
	}

	tset := c.Runbook().TaskSet()
	if tset.Has("clean") {
		return tset.Run(ctx, "clean")
	}
	return runbook.ErrUnknownTask
}

func (l *Load) Error() string {

	var str strings.Builder

	fmt.Fprintf(&str, "\n\x1b[0;34m❯❯❯❯❯❯❯❯❯❯❯❯  %s\x1b[0m\n%s", l.carton, l.err) // blue(34)
	if l.buf != nil && l.buf.Len() > 0 {
		str.WriteString(fmt.Sprintf("\n\n\x1b[0;31m%s \x1b[0m", "Error log: ↡\n")) // red(31)
		str.Write(l.buf.Bytes())
	}
	return str.String()
}

func setupArg(carton carton.Builder, arg *runbook.Arg) {

	arg.Owner = carton.Provider()
	arg.FilesPath = carton.FilesPath()
	arg.Wd = WorkDir(carton, false)
	arg.SrcDir = carton.SrcPath
	arg.VisitVars = func(fn func(key, value string)) {
		carton.VisitVars(fn)
	}
	arg.LookupVar = func(key string) (string, bool) {
		if value, ok := arg.Vars[key]; ok {
			return value, ok
		}
		return carton.LookupVar(key)
	}

	TOPDIR := config.GetVar(config.TOPDIR)
	arg.Vars = map[string]string{
		"WORKDIR": WorkDir(carton, false),
		"TOPDIR":  TOPDIR,

		"PN": carton.Provider(),
		"S":  carton.SrcPath(arg.Wd),
		"T":  filepath.Join(arg.Wd, "temp"),
		"D":  filepath.Join(arg.Wd, "image"),

		"IMAGEDIR": filepath.Join(TOPDIR, config.GetVar(config.IMAGEDIR),
			config.GetVar(config.MACHINE)),
		"STAGINGDIR": filepath.Join(TOPDIR, config.GetVar(config.STAGINGDIR)),

		"TARGETARCH":   getTargetArch(carton, false),
		"TARGETOS":     getTargetOS(carton, false),
		"TARGETVENDOR": getTargetVendor(carton, false),
	}

}

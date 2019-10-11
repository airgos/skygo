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
	"merge/runbook/xsync"
)

// Load represent state of load
type Load struct {
	works int
	pool  *xsync.Pool

	arg  []*runbook.Arg
	bufs []*bytes.Buffer

	cancel context.CancelFunc

	// loadError is allowed to set only once
	err  loadError
	once sync.Once
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
// num represent how many loader work. if its value is 0, it will use default value
func NewLoad(num int) *Load {

	if num == 0 {
		num = runtime.NumCPU()
	}
	load := Load{
		arg:   make([]*runbook.Arg, num),
		bufs:  make([]*bytes.Buffer, num),
		works: num,
	}
	load.pool = xsync.NewPool(num, func(i int) interface{} {
		x := pool{
			arg: new(runbook.Arg),
			buf: new(bytes.Buffer),
		}

		x.arg.SetUnderOutput(nil, x.buf)
		load.arg[i] = x.arg
		load.bufs[i] = x.buf
		return &x
	})

	return &load
}

// SetOutput assign stdout & stderr for one load
func (l *Load) SetOutput(index int, stdout, stderr io.Writer) *Load {
	if index >= l.works {
		return nil
	}

	l.arg[index].SetUnderOutput(stdout,
		io.MultiWriter(stderr, l.bufs[index]))
	return l
}

func (l *Load) perform(ctx context.Context, c carton.Builder, target string,
	nodeps bool) (err error) {

	x := l.pool.Get(ctx).(*pool)
	defer l.pool.Put(x)

	setupArg(c, x.arg)
	if nodeps {
		x.arg.SetUnderOutput(os.Stdout, os.Stderr)
	}
	ctx = runbook.NewContext(ctx, x.arg)

	// reset buffer
	x.buf.Reset()

	// mkdir temp
	os.MkdirAll(x.arg.Vars["T"], 0755)

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

func (l *Load) run(ctx context.Context, name, target string) {
	var wg sync.WaitGroup

	b, _, err := carton.Find(name)
	if err != nil {
		l.err = loadError{
			carton: name,
			err:    err,
		}
		return
	}
	SetupRunbook(b.Runbook())

	scan := func(deps []string) {
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
	}

	scan(b.BuildDepends())
	scan(b.Depends())
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
			l.err = loadError{
				carton: name,
				err:    err,
			}
			return &l.err
		}
		SetupRunbook(b.Runbook())
		return l.perform(ctx, b, target, true)
	}

	l.run(ctx, name, target)
	if l.err.err != nil {
		return &l.err
	}
	return nil
}

// Clean invokes carton's method Clean
func (l *Load) Clean(ctx context.Context, name string, force bool) error {

	c, _, err := carton.Find(name)
	if err != nil {
		l.err = loadError{
			carton: name,
			err:    err,
		}
		return &l.err
	}

	if force {
		os.RemoveAll(WorkDir(c, false))
		return nil
	}

	// TODO: only run if FETCH was performed successfully
	return l.perform(ctx, c, "clean", true)
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

func SetupRunbook(rb *runbook.Runbook) {

	if s := rb.Stage(carton.PATCH); s != nil {
		s.AddTask(0, func(ctx context.Context) error {
			return patch(ctx)
		})
	}
	addEventListener(rb)
}

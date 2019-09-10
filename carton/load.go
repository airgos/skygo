// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package carton

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"

	"merge/runbook"
)

// Load represent state of load
type Load struct {
	works int
	ch    chan int

	arg  []*runbook.Arg
	bufs []*bytes.Buffer

	cancel context.CancelFunc

	// err is allowed to set only once
	once  sync.Once
	err   error
	index int // which load occurs error
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
// It's not safe to invoke during loading
func (l *Load) SetOutput(index int, stdout, stderr io.Writer) *Load {
	l.arg[index].SetOutput(stdout,
		io.MultiWriter(stderr, l.bufs[index]))
	return l
}

func (l *Load) perform(ctx context.Context, carton Builder, target string,
	nodeps bool) (index int, err error) {

	index = l.get()
	arg := l.arg[index]
	arg.Owner = carton.Provider()
	arg.Direnv = carton.(runbook.DirEnv)

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
	return index, err
}

func (l *Load) run(ctx context.Context, carton, target string) {
	var wg sync.WaitGroup

	b, _, err := Find(carton)
	if err != nil {
		l.err = err
		return
	}

	deps := b.BuildDepends()
	required := b.Depends()
	deps = append(deps, required...)

	wg.Add(len(deps))
	for _, d := range deps {
		carton := d
		target := ""
		if i := strings.LastIndex(d, "@"); i >= 0 {
			carton, target = d[:i], d[i+1:]
		}
		go func(ctx context.Context, carton, target string) {

			select {
			default:
				l.run(ctx, carton, target)
			case <-ctx.Done():
			}
			wg.Done()
		}(ctx, carton, target)
	}
	wg.Wait()

	if index, err := l.perform(ctx, b, target, false); err != nil {
		l.cancel()
		l.once.Do(func() {
			l.err = fmt.Errorf("%s==>\n%s", b.Provider(), err)
			l.index = index
		})
	}
}

// Run start loading
func (l *Load) Run(ctx context.Context, carton, target string, nodeps bool) error {
	ctx, cancel := context.WithCancel(ctx)
	l.cancel = cancel

	if nodeps {

		b, _, err := Find(carton)
		if err != nil {
			return err
		}
		_, err = l.perform(ctx, b, target, true)
		return err
	}

	l.run(ctx, carton, target)
	if l.err != nil {
		return l
	}
	return nil
}

func (l *Load) Error() string {

	var str strings.Builder
	fmt.Fprintln(&str, l.err)
	fmt.Fprintf(&str, "\nError log:\n")
	str.Write(l.bufs[l.index].Bytes())
	return str.String()
}

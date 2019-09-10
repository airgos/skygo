// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package carton

import (
	"context"
	"io"
	"runtime"
	"strings"
	"sync"

	"merge/runbook"
)

// Load represent state of load
type Load struct {
	ch     chan *runbook.Arg
	arg    []*runbook.Arg
	works  int
	cancel context.CancelFunc

	// err is allowed to set only once
	once sync.Once
	err  error
}

// NewLoad create load to build carton
// num represent how many loader work. if its value is 0, it will use default value
func NewLoad(num int) *Load {

	if num == 0 {
		num = runtime.NumCPU()
	}
	load := Load{
		ch:    make(chan *runbook.Arg, num),
		arg:   make([]*runbook.Arg, num),
		works: num,
	}
	for i := 0; i < num; i++ {
		arg := new(runbook.Arg)
		load.arg[i] = arg
		load.ch <- arg
	}

	return &load
}

func (l *Load) get() *runbook.Arg {
	return <-l.ch
}

func (l *Load) put(arg *runbook.Arg) {
	l.ch <- arg
}

// SetOutput assign stdout & stderr for one load
// It's not safe to invoke during loading
func (l *Load) SetOutput(index int, stdout, stderr io.Writer) *Load {
	l.arg[index].SetOutput(stdout, stderr)
	return l
}

func (l *Load) perform(ctx context.Context, carton Builder, target string, nodeps bool) (err error) {

	arg := l.get()
	arg.Owner = carton.Provider()
	arg.Direnv = carton.(runbook.DirEnv)

	ctx = runbook.NewContext(ctx, arg)

	if nodeps && target != "" {
		err = carton.Runbook().Play(ctx, target)
	} else {
		err = carton.Runbook().Perform(ctx, target)
	}
	l.put(arg)
	return
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

	err = l.perform(ctx, b, target, false)
	if err != nil {
		l.cancel()
		if l.err == nil {
			l.once.Do(func() {
				l.err = err
			})
		}
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
		return l.perform(ctx, b, target, true)
	}

	l.run(ctx, carton, target)
	return l.err
}

// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package carton

import (
	"context"
	"io"
	"runtime"
	"sync"

	"merge/runbook"
)

// Load represent state of load
type Load struct {
	ch     chan *runbook.Arg
	arg    []*runbook.Arg
	carton string
	works  int
	cancel context.CancelFunc
	err    error
}

// NewLoad create load to build carton
// num represent how many loader work. if its value is 0, it will use default value
func NewLoad(num int, carton string) *Load {

	if num == 0 {
		num = runtime.NumCPU()
	}
	load := Load{
		ch:     make(chan *runbook.Arg, num),
		arg:    make([]*runbook.Arg, num),
		carton: carton,
		works:  num,
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
	l.arg[index].Stderr = stderr
	l.arg[index].Stdout = stdout
	return l
}

func (l *Load) run(ctx context.Context, carton string) {
	var wg sync.WaitGroup

	b, _, _ := Find(carton)
	deps := b.BuildDepends()
	required := b.Depends()
	deps = append(deps, required...)

	wg.Add(len(deps))
	for _, d := range deps {
		go func(ctx context.Context, carton string) {

			select {
			default:
				l.run(ctx, carton)
			case <-ctx.Done():
			}
			wg.Done()
		}(ctx, d)
	}
	wg.Wait()

	arg := l.get()
	arg.Owner = carton
	arg.Direnv = b.(runbook.DirEnv)

	ctx = runbook.NewContext(ctx, arg)
	e := b.Runbook().Perform(ctx)
	l.put(arg)
	if e != nil {
		l.cancel()
		if l.err == nil {
			// BUG: l.err can be overwritten
			l.err = e
		}
	}
}

// Run start loading
func (l *Load) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	l.cancel = cancel
	l.run(ctx, l.carton)
	return l.err
}

// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package carton

import (
	"context"
	"io"
	"os"
	"runtime"
	"sync"

	"merge/runbook"
)

// Load represent state of load
type Load struct {
	ch     chan *resource
	res    []*resource
	carton string
	works  int
	cancel context.CancelFunc
	err    error
}

type resource struct {
	stdout, stderr io.Writer
}

// NewLoad create load to build carton
// num represent how many loader work. if its value is 0, it will use default value
func NewLoad(num int, carton string) *Load {

	if num == 0 {
		num = runtime.NumCPU()
	}
	load := Load{
		ch:     make(chan *resource, num),
		res:    make([]*resource, num),
		carton: carton,
		works:  num,
	}
	for i := 0; i < num; i++ {
		res := new(resource)
		load.res[i] = res
		load.ch <- res
	}

	return &load
}

func (l *Load) get() *resource {
	return <-l.ch
}

func (l *Load) put(res *resource) {
	l.ch <- res
}

// SetOutput assign stdout & stderr for one load
// It's not safe to invoke during loading
func (l *Load) SetOutput(index int, stdout, stderr io.Writer) *Load {
	l.res[index].stderr = stderr
	l.res[index].stdout = stdout
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
	res := l.get()

	ctx = runbook.CtxWithOutput(ctx, os.Stdout, os.Stderr)
	e := b.Runbook().Perform(ctx)
	l.put(res)
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

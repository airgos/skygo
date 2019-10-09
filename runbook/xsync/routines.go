// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xsync

import (
	"context"
	"sync"
)

// A RoutineGroup is a collection of goroutines working on subtasks that are part of
// the same overall task.
type RoutineGroup struct {
	cancel  func()
	wg      sync.WaitGroup
	errOnce sync.Once
	err     error
}

// WithConext return a new context and an associated ctx derived ctx
func WithContext(ctx context.Context) (*RoutineGroup, context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	return &RoutineGroup{cancel: cancel}, ctx
}

// Go calls the given function in a new goroutine.
func (g *RoutineGroup) Go(fn func() error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if err := fn(); err != nil {
			g.errOnce.Do(func() {
				g.err = err
				if g.cancel != nil {
					g.cancel()
				}
			})
		}
	}()
}

// Wait watis all routines finished
func (g *RoutineGroup) Wait() error {

	g.wg.Wait()
	if g.cancel != nil {
		g.cancel()
	}
	return g.err
}

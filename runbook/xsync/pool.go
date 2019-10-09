// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//Package xsync provides:
//   number pool with synchronization
//   routine group
package xsync

import (
	"container/list"
	"context"
	"sync"
)

type Pool struct {
	m    sync.Mutex
	head list.List

	ready chan interface{}
}

// newPool creates resource pool
func NewPool(size int, New func(index int) interface{}) *Pool {

	p := Pool{
		ready: make(chan interface{}),
	}
	for i := 0; i < size; i++ {
		x := New(i)
		p.head.PushBack(x)
	}
	return &p
}

// get acquire one from pool
func (p *Pool) Get(ctx context.Context) interface{} {

	p.m.Lock()
	if p.head.Len() != 0 {
		x := p.head.Front()
		p.m.Unlock()
		return x.Value
	}
	p.m.Unlock()
	select {
	case <-ctx.Done():
		return nil

	case x := <-p.ready:
		return x
	}
}

// put push back to pool
func (p *Pool) Put(x interface{}) {
	p.m.Lock()
	defer p.m.Unlock()

	if p.head.Len() == 0 {
		p.ready <- x
		return
	}
	p.head.PushFront(x)
}

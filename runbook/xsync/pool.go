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
	m       sync.Mutex
	head    list.List
	waiters list.List
}

// newPool creates resource pool
func NewPool(size int, New func(index int) interface{}) *Pool {

	p := Pool{}
	for i := 0; i < size; i++ {
		x := New(i)
		p.head.PushBack(x)
	}
	return &p
}

// get acquire one from pool
func (p *Pool) Get(ctx context.Context) (interface{}, error) {

	p.m.Lock()
	if p.head.Len() != 0 {
		elem := p.head.Front()
		p.head.Remove(elem)
		p.m.Unlock()
		return elem.Value, nil
	}
	ready := make(chan struct{})
	elem := p.waiters.PushBack(ready)
	p.m.Unlock()
	select {
	case <-ctx.Done():
		p.waiters.Remove(elem)
		return nil, ctx.Err()

	case <-ready:
		p.m.Lock()
		elem := p.head.Front()
		p.head.Remove(elem)
		p.m.Unlock()
		return elem.Value, nil
	}
}

// put push back to pool
func (p *Pool) Put(x interface{}) {
	p.m.Lock()
	defer p.m.Unlock()
	p.head.PushBack(x)

	if p.waiters.Len() != 0 {
		elem := p.waiters.Front()
		ready := elem.Value.(chan struct{})
		close(ready)
		p.waiters.Remove(elem)
	}
}

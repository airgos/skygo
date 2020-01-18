// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runbook

import (
	"container/list"
)

// event listeners
// it's embedded by others, like Stage, independent TaskSet
type listeners struct {
	reset list.List
	inout list.List
}

type inout struct {
	// after calling enter, return x should be save to member x
	// if handled is true, caller should stop moving forward
	enter func(ctx Context, name string) (handled bool, x interface{}, err error)

	// input paramter is from member x
	exit func(ctx Context, name string, x interface{}) error

	x interface{}
}

// PushInOut push listener enter & exit
func (l *listeners) PushInOut(
	enter func(ctx Context, name string) (handled bool, x interface{}, err error),
	exit func(ctx Context, name string, x interface{}) error) {

	pair := inout{
		enter: enter,
		exit:  exit,
	}
	l.inout.PushBack(&pair)
}

// rangeIn interates to call listener enter
// If any listener enter returns handled or error, RangeIn stops the iteration.
func (l *listeners) rangeIn(ctx Context, name string) (handled bool, err error) {

	for e := l.inout.Front(); e != nil; e = e.Next() {
		listener := e.Value.(*inout)
		if listener.enter == nil {
			continue
		}
		handled, x, err := listener.enter(ctx, name)
		if handled || err != nil {
			return handled, err
		}
		listener.x = x
	}
	return
}

// rangeOut interates to call listener exit
// If any listener exit returns error, RangeIn stops the iteration.
func (l *listeners) rangeOut(ctx Context, name string) error {

	for e := l.inout.Front(); e != nil; e = e.Next() {
		listener := e.Value.(*inout)
		if listener.exit == nil {
			continue
		}
		if err := listener.exit(ctx, name, listener.x); err != nil {
			return err
		}
	}
	return nil
}

// PushReset push listener reset
func (l *listeners) PushReset(reset func(ctx Context, name string) error) {

	l.reset.PushBack(reset)
}

// rangeReset interates to call listener reset
func (l *listeners) rangeReset(ctx Context, name string) error {

	for e := l.reset.Front(); e != nil; e = e.Next() {
		reset := e.Value.(func(ctx Context, name string) error)
		if reset == nil {
			continue
		}
		if err := reset(ctx, name); err != nil {
			return err
		}
	}
	return nil
}

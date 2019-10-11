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
	enter func(name string, arg *Arg) (handled bool, x interface{}, err error)

	// input paramter is from member x
	exit func(name string, arg *Arg, x interface{}) error

	x interface{}
}

// PushInOut push listener enter & exit
func (l *listeners) PushInOut(
	enter func(name string, arg *Arg) (handled bool, x interface{}, err error),
	exit func(name string, arg *Arg, x interface{}) error) {

	pair := inout{
		enter: enter,
		exit:  exit,
	}
	l.inout.PushBack(&pair)
}

// RangeIn interates to call listener enter
// If any listener enter returns handled or error, RangeIn stops the iteration.
func (l *listeners) RangeIn(name string, arg *Arg) (handled bool, err error) {

	for e := l.inout.Front(); e != nil; e = e.Next() {
		listener := e.Value.(*inout)
		if listener.enter == nil {
			continue
		}
		handled, x, err := listener.enter(name, arg)
		if handled || err != nil {
			return handled, err
		}
		listener.x = x
	}
	return
}

// RangeOut interates to call listener exit
// If any listener exit returns error, RangeIn stops the iteration.
func (l *listeners) RangeOut(name string, arg *Arg) error {

	for e := l.inout.Front(); e != nil; e = e.Next() {
		listener := e.Value.(*inout)
		if listener.exit == nil {
			continue
		}
		if err := listener.exit(name, arg, listener.x); err != nil {
			return err
		}
	}
	return nil
}

// PushReset push listener reset
func (l *listeners) PushReset(reset func(name string, arg *Arg) error) {

	l.reset.PushBack(reset)
}

// RangeReset interates to call listener reset
func (l *listeners) RangeReset(name string, arg *Arg) error {

	for e := l.reset.Front(); e != nil; e = e.Next() {
		reset := e.Value.(func(name string, arg *Arg) error)
		if reset == nil {
			continue
		}
		if err := reset(name, arg); err != nil {
			return err
		}
	}
	return nil
}

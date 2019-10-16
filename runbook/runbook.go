// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runbook

import (
	"container/list"
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"

	"merge/log"
)

// Error used by Runbook
var (
	ErrTaskAdded       = errors.New("Task Added")
	ErrUnknownTaskType = errors.New("Unkown Task Type")
	ErrNilStage        = errors.New("Nil Stage")
	ErrUnknownTask     = errors.New("Unkown Task")
)

// Runbook consists of a series of stage and a independent taskset
type Runbook struct {
	head *list.List

	// inherits event listeners
	// it's for independent TaskSet
	listeners

	//independent TaskSet
	taskset *TaskSet
}

// Stage is member of Runbook, and hold a set of tasks with differnt weight.
// task with the lowest weight is executed first.
type Stage struct {
	name string

	tasks *TaskSet

	e       *list.Element
	runbook *Runbook

	m        sync.Mutex
	executed uint32 // executed ?

	// inherits event listeners
	listeners
}

// NewRunbook create a Runbook
func NewRunbook() *Runbook {
	this := new(Runbook)
	this.head = list.New()
	this.taskset = newTaskSet()
	this.listeners.inout.Init()
	this.listeners.reset.Init()
	return this
}

// RunbookInfo give stage slice with the number of task, independent task names
func (rb *Runbook) RunbookInfo() ([]string, []int, []string) {

	num := rb.head.Len()
	stages := make([]string, 0, num)
	tasknum := make([]int, 0, num)

	for stage := rb.Head(); stage != nil; stage = stage.Next() {
		stages = append(stages, stage.name)
		tasknum = append(tasknum, stage.tasks.Len())
	}
	taskname := []string{}
	for n := range rb.taskset.set {
		taskname = append(taskname, n.(string))
	}
	return stages, tasknum, taskname
}

// PushBack new a stage, and push at the end
func (rb *Runbook) PushBack(name string) *Stage {

	stage := newStage(name)
	stage.runbook = rb
	stage.e = rb.head.PushBack(stage)

	return stage
}

// PushFront new a stage, and push at the front
func (rb *Runbook) PushFront(name string) *Stage {

	stage := newStage(name)
	stage.runbook = rb
	stage.e = rb.head.PushFront(stage)

	return stage
}

// Stage find stage struct by @name
func (rb *Runbook) Stage(name string) (stage *Stage) {

	l := rb.head
	for e := l.Front(); e != nil; e = e.Next() {
		stage = e.Value.(*Stage)
		if stage.name == name {
			return stage
		}
	}
	return nil
}

// Head return the first stage in the runbook
func (rb *Runbook) Head() *Stage {

	l := rb.head
	if e := l.Front(); e != nil {
		return e.Value.(*Stage)

	}
	return nil
}

// TaskSet given indelete takset owned by runbook
func (rb *Runbook) TaskSet() *TaskSet {
	return rb.taskset
}

// RunTask play one task in dependent taskset
func (rb *Runbook) RunTask(ctx context.Context, name string) error {

	log.Trace("Run independent task: %s", name)
	arg, _ := FromContext(ctx)
	if handled, err := rb.RangeIn(name, arg); handled || err != nil {
		return err
	}

	if err := rb.taskset.Run(ctx, name); err != nil {
		return err
	}
	return rb.RangeOut(name, arg)
}

// Range iterates all stages and execute Play in the runbook
// Abort if any stage failed
// After invoking Play, abort if current stage is @name
func (rb *Runbook) Range(ctx context.Context, name string) error {

	for stage := rb.Head(); stage != nil; stage = stage.Next() {
		if stage.tasks.Len() > 0 {

			err := stage.Play(ctx)
			if err != nil {
				return err
			}

			if stage.name == name {
				return nil
			}
		}
	}
	return nil
}

// Play run stage's tasks or the independent task
func (rb *Runbook) Play(ctx context.Context, name string) error {

	arg, _ := FromContext(ctx)
	log.Trace("Play stage or task %s held by %s", name, arg.Owner)

	if s := rb.Stage(name); s != nil {
		return s.Play(ctx)
	}
	return rb.RunTask(ctx, name)
}

func newStage(name string) *Stage {

	stage := Stage{
		name:  name,
		tasks: newTaskSet(),
	}

	// init event listeners
	stage.listeners.inout.Init()
	stage.listeners.reset.Init()

	stage.tasks.routine = name
	return &stage
}

// Name give the name of the stage
func (s *Stage) Name() string {
	return s.name
}

// InsertAfter insert a new stage @name after current one
// Return new stage
func (s *Stage) InsertAfter(name string) *Stage {

	n := newStage(name)
	n.runbook = s.runbook
	n.e = s.runbook.head.InsertAfter(n, s.e)

	return n
}

// InsertBefore insert a new stage @name before current one
// Return new stage
func (s *Stage) InsertBefore(name string) *Stage {

	n := newStage(name)
	n.runbook = s.runbook
	n.e = s.runbook.head.InsertBefore(n, s.e)

	return n
}

// Next stage
func (s *Stage) Next() *Stage {

	if e := s.e.Next(); e != nil {

		return e.Value.(*Stage)
	}
	return nil
}

// AddTask add one task with weight
func (s *Stage) AddTask(weight int, task interface{}) (*Stage, error) {

	if s == nil {
		return nil, ErrNilStage
	}

	_, err := s.tasks.Add(weight, task)
	return s, err
}

// DelTask delete task of weight
func (s *Stage) DelTask(weight int) {

	if s == nil {
		return
	}
	s.tasks.Del(weight)
}

// Reset clear executed status, then s.Play can be run again
func (s *Stage) Reset(ctx context.Context) {

	arg, _ := FromContext(ctx)
	s.RangeReset(s.name, arg)

	s.m.Lock()
	defer s.m.Unlock()
	atomic.StoreUint32(&s.executed, 0)
}

// Play perform tasks in the stage
func (s *Stage) Play(ctx context.Context) error {

	if atomic.LoadUint32(&s.executed) == 1 {
		return nil
	}

	s.m.Lock()
	defer s.m.Unlock()
	if s.executed == 0 {

		defer atomic.StoreUint32(&s.executed, 1)

		arg, _ := FromContext(ctx)
		if handled, err := s.RangeIn(s.name, arg); handled || err != nil {
			return err
		}

		if err := s.tasks.Play(ctx); err != nil {
			return err
		}
		return s.RangeOut(s.name, arg)
	}
	return nil
}

// Arg holds arguments for runbook
type Arg struct {
	// who own this, same as LookupVar("PN")
	Owner string

	// FilesPath is a collection of directory that's be used for locating local file
	FilesPath []string

	// value of WORKDIR, same as LookupVar("WORKDIR")
	Wd string

	// SrcDir calculate Source Dir under WORKDIR
	SrcDir func(wd string) string

	// Visit each variable and export to command task
	// it shound not range Vars
	VisitVars func(func(key, value string))

	// LookupVar retrieves the value of the variable named by the key.
	// If the variable is present, value (which may be empty) is returned
	// and the boolean is true. Otherwise the returned value will be empty
	// and the boolean will be false.
	// golang task should call it to get value of Variable
	LookupVar func(key string) (string, bool)

	// LookupVar implementation should check Vars firstly, if it does not exist, try other
	Vars map[string]string

	// underline IO, call method Output() to get IO
	stdout, stderr io.Writer

	// help to wrap IO based on underline IO
	// example: use io.MultiWriter to duplicates its writes
	Writer func() (stdout, stderr io.Writer)

	m sync.Mutex
}

// Output return IO stdout & stderr
func (arg *Arg) Output() (stdout, stderr io.Writer) {
	arg.m.Lock()
	defer arg.m.Unlock()
	if arg.Writer != nil {
		return arg.Writer()
	}
	return arg.stdout, arg.stderr
}

// UnderOutput return IO stdout & stderr
func (arg *Arg) UnderOutput() (stdout, stderr io.Writer) {
	return arg.stdout, arg.stderr
}

// SetUnderOutput set underline IO stdout & stderr
func (arg *Arg) SetUnderOutput(stdout, stderr io.Writer) {
	arg.m.Lock()
	arg.stdout, arg.stderr = stdout, stderr
	arg.m.Unlock()
}

type argToken string

// NewContext returns a new Context that carries value arg
func NewContext(ctx context.Context, arg *Arg) context.Context {

	return context.WithValue(ctx, argToken("arg"), arg)
}

// FromContext returns the Arg value stored in ctx, if any
func FromContext(ctx context.Context) (*Arg, bool) {
	arg, ok := ctx.Value(argToken("arg")).(*Arg)
	return arg, ok
}

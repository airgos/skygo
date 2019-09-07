// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runbook

import (
	"container/list"
	"context"
	"errors"
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
	head    *list.List
	taskset *TaskSet
	runtime Runtime
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
}

// NewRunbook create a Runbook
func NewRunbook(runtime Runtime) *Runbook {
	this := new(Runbook)
	this.head = list.New()
	this.runtime = runtime
	this.taskset = newTaskSet()
	return this
}

// Clone clone Runbook w/ different runtime
func (rb *Runbook) Clone(runtime Runtime) *Runbook {
	n := Runbook{
		head:    rb.head,
		taskset: rb.taskset,
		runtime: runtime,
	}
	return &n
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

	stage := newStage(name, rb.runtime)
	stage.runbook = rb
	stage.e = rb.head.PushBack(stage)

	return stage
}

// PushFront new a stage, and push at the front
func (rb *Runbook) PushFront(name string) *Stage {

	stage := newStage(name, rb.runtime)
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
			break
		}
	}
	return
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
	return rb.taskset.Run(ctx, name, rb.runtime)
}

// Perform carry out all stages in the runbook
// Break if any stage failed
func (rb *Runbook) Perform(ctx context.Context) error {

	for stage := rb.Head(); stage != nil; stage = stage.Next() {
		if stage.tasks.Len() > 0 {

			err := stage.Play(ctx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Play run stage's tasks or the independent task
func (rb *Runbook) Play(ctx context.Context, name string) error {

	log.Trace("Play stage or task %s", name) // TODO: which carton
	if s := rb.Stage(name); s != nil {
		return s.Play(ctx)
	}
	return rb.RunTask(ctx, name)
}

func newStage(name string, runtime Runtime) *Stage {

	stage := Stage{
		name:  name,
		tasks: newTaskSet(),
	}

	stage.tasks.routine = name
	return &stage
}

// InsertAfter insert a new stage @name after current one
// Return new stage
func (s *Stage) InsertAfter(name string) *Stage {

	n := newStage(name, s.runbook.runtime)
	n.runbook = s.runbook
	n.e = s.runbook.head.InsertAfter(n, s.e)

	return n
}

// InsertBefore insert a new stage @name before current one
// Return new stage
func (s *Stage) InsertBefore(name string) *Stage {

	n := newStage(name, s.runbook.runtime)
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

// Play perform tasks in the stage
func (s *Stage) Play(ctx context.Context) error {

	if atomic.LoadUint32(&s.executed) == 1 {
		return nil
	}

	// TODO:
	// read record to check whether it's executed last time
	// Need: vaule of PWD

	s.m.Lock()
	defer s.m.Unlock()
	if s.executed == 0 {
		defer atomic.StoreUint32(&s.executed, 1)
		return s.tasks.Play(ctx, s.runbook.runtime)
	}
	return nil
}

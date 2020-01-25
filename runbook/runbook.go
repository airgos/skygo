// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runbook

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"

	"skygo/utils/log"
)

// Context define methods used by runbook
type Context interface {

	// who own this Context & runbook
	Owner() string

	//interface to retrieves data from the context.
	KVGetter

	//GetStr retrieves data from the context.
	//just a wrapper of KVGetter.Get
	GetStr(key string) string

	// saves data in the context.
	// each Context should has its own KV instance for setting
	KVSetter

	// give stdout & stderr IO
	Output() (stdout, stderr io.Writer)

	// give file path for locating general file, e.g. patch file
	FilesPath() []string

	// retrieves SRC dir and build dir from context
	// build dir is combination of SRC dir and env B when B is relative path
	// if no B, SRC dir is the same as build dir
	//
	// B should be configured at carton level regardless of native attribute
	// e.g. wget.Set("B", "build")
	Dir() (srcDir, buildDir string)

	// retrieves parent standard context
	Ctx() context.Context

	// Wait waits one dependent stage belong to another runbook finished
	// upon stage struct is got, Wait has to call stage's method RegisterNotifier to add notifier
	// if notifier is not nil
	Wait(runbook, stage string, isNative bool, notifier func(Context)) <-chan struct{}

	// private data
	Private() interface{}

	// acquire permission to run
	Acquire() error

	//  release permission
	Release()
}

// Error used by Runbook
var (
	ErrUnknownTaskType = errors.New("Unkown Task Type")
	ErrUnknownTask     = errors.New("Unkown Task")
)

// Runbook consists of a series of stage and a task force
type Runbook struct {
	head *list.List

	// inherits event listeners
	// it's for TaskForce
	listeners

	taskForce map[string]*TaskForce
}

type stageDep struct {
	runbook  string //format: runbookName[@stageName]
	notifier func(Context)
}

// Stage is member of Runbook, and hold a set of tasks with differnt weight.
// task with the lowest weight is executed first.
type Stage struct {
	name string
	help string

	taskset *TaskSet

	e       *list.Element
	runbook *Runbook

	m        sync.Mutex
	executed [2]uint32 // executed ?
	disabled bool

	depends  []stageDep
	notifier list.List

	ready [2]chan struct{}

	// inherits event listeners
	listeners
}

// NewRunbook create a Runbook
func NewRunbook() *Runbook {
	this := new(Runbook)
	this.head = list.New()
	this.taskForce = make(map[string]*TaskForce)
	this.listeners.inout.Init()
	this.listeners.reset.Init()
	return this
}

// String output information stages and task force
func (rb *Runbook) String() string {

	var s strings.Builder

	// output stage information
	if head := rb.Head(); head != nil {

		disabled := func(stage *Stage) string {
			if stage.disabled {
				return "disabled, "
			}
			return ""
		}
		fmt.Fprintf(&s, "\n%13s: %s[%s%d]", "Stage Flow", head.name, disabled(head), head.taskset.Len())
		for stage := head.Next(); stage != nil; stage = stage.Next() {
			fmt.Fprintf(&s, " ->> %s[%s%d]", stage.name, disabled(stage), stage.taskset.Len())
		}
		fmt.Fprintf(&s, "\nStage Summary:\n")

		for stage := head; stage != nil; stage = stage.Next() {
			fmt.Fprintf(&s, "%13s: %s\n", stage.name, stage.help)
		}
	}

	fmt.Fprintf(&s, "\nTask Force\n")
	for name, tf := range rb.taskForce {
		fmt.Fprintf(&s, "%10s: %s\n", name, tf.summary)
	}

	return s.String()
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

// NewTaskForce new task force
// it supports two kind of task: TaskGoFunc & script. script is a script file
// name or string. if it's a script file, task runner will try to find it under
// FilesPath
func (rb *Runbook) NewTaskForce(name string, task interface{}, summary string) *TaskForce {
	tf := newTaskForce(name, summary)
	tf.setTask(task)
	rb.taskForce[name] = tf
	return tf
}

// HasTaskForce return whether runbook has task force @name
func (rb *Runbook) HasTaskForce(name string) bool {

	_, ok := rb.taskForce[name]
	return ok
}

// runTaskForce run task in task force
func (rb *Runbook) runTaskForce(ctx Context, name string) error {

	isNative := ctx.Get("ISNATIVE").(bool)

	tf := rb.taskForce[name]

	// wait its dependent stages belong to another rubooks are finished
	for _, d := range tf.depends {

		runbook := d
		stage := ""
		if i := strings.LastIndex(d, "@"); i >= 0 {
			runbook = d[:i]
			stage = d[i+1:]
		}

		select {
		case <-ctx.Ctx().Done():
			return ctx.Ctx().Err()
		case <-ctx.Wait(runbook, stage, isNative, nil):
		}
	}

	log.Trace("Run task force owned by %s: %s", ctx.Owner(), name)
	if handled, err := rb.rangeIn(ctx, name); handled || err != nil {
		return err
	}

	// TaskForce only have one task
	if err := tf.taskset.runtask(ctx, 0); err != nil {
		return err
	}
	return rb.rangeOut(ctx, name)
}

// Play run task force or iterates stages until stage @target
// if @target is emptry, it will iterates all stages
func (rb *Runbook) Play(ctx Context, target string) error {

	if rb.HasTaskForce(target) {
		return rb.runTaskForce(ctx, target)
	}

	if target != "" && nil == rb.Stage(target) {
		return fmt.Errorf("%s has no stage or task force %s", ctx.Owner(), target)
	}

	log.Trace("Range stages held by %s", ctx.Owner())
	for stage := rb.Head(); stage != nil; stage = stage.Next() {

		if num := stage.taskset.Len(); num > 0 {

			if err := ctx.Acquire(); err != nil {
				return err
			}
			log.Trace("Play stage %s[tasks=%d] held by %s",
				target, num, ctx.Owner())

			err := stage.play(ctx)
			if err != nil {
				return err
			}

			ctx.Release()
			if stage.name == target {
				return nil
			}
		}
	}
	return nil
}

func newStage(name string) *Stage {

	stage := Stage{
		name:    name,
		taskset: newTaskSet(),
	}

	stage.ready[0] = make(chan struct{})
	stage.ready[1] = make(chan struct{})

	// init event listeners
	stage.listeners.inout.Init()
	stage.listeners.reset.Init()

	stage.taskset.owner = name
	return &stage
}

// Name give the name of the stage
func (s *Stage) Name() string {
	return s.name
}

// Disable makrs Play is not allowed to be run
func (s *Stage) Disable() *Stage {

	s.m.Lock()
	s.disabled = true
	s.m.Unlock()
	return s
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

// Summary sets help message
func (s *Stage) Summary(summary string) *Stage {
	s.help = summary
	return s
}

// Next stage
func (s *Stage) Next() *Stage {

	if e := s.e.Next(); e != nil {

		return e.Value.(*Stage)
	}
	return nil
}

// AddTask add one task with weight to stage's taskset
func (s *Stage) AddTask(weight int, task interface{}) *Stage {

	s.taskset.Add(weight, task)
	return s
}

// DelTask delete task of weight from stage's taskset
func (s *Stage) DelTask(weight int) *Stage {

	if s == nil {
		return nil
	}
	s.taskset.Del(weight)
	return s
}

// Reset clear executed status, then s.Play can be run again
func (s *Stage) Reset(ctx Context) {

	s.rangeReset(ctx, s.name)

	s.m.Lock()
	defer s.m.Unlock()

	isNative := ctx.Get("ISNATIVE").(bool)
	atomic.StoreUint32(&s.executed[index(isNative)], 0)
}

// AddDep add one dependent stage who are belong to another runbooks to current stage.
// format of parameter @d: runbookName[@stageName]
func (s *Stage) AddDep(d string, notifier func(Context)) *Stage {
	s.m.Lock()
	defer s.m.Unlock()

	s.depends = append(s.depends, stageDep{
		runbook:  d,
		notifier: notifier,
	})
	return s
}

// registerNotifier push one notifier callback to notifier chain
// notifier chain is iterated after stage is executed
func (s *Stage) registerNotifier(n func(Context)) *Stage {

	s.m.Lock()
	defer s.m.Unlock()
	s.notifier.PushBack(n)
	return s
}

// iterating nofitifier chain and delete items
func (s *Stage) callNotifierChain(ctx Context) {

	if ctx == nil {
		return
	}

	for e := s.notifier.Front(); e != nil; e = e.Next() {
		n := e.Value.(func(Context))
		n(ctx)
		s.notifier.Remove(e)
	}
}

func index(isNative bool) int {

	index := 0
	if isNative {
		index = 1
	}
	return index
}

// Wait return channel for waiting this stage is finished
func (s *Stage) Wait(ctx Context, notifier func(Context), isNative bool) <-chan struct{} {

	if notifier != nil {
		s.registerNotifier(notifier)
	}

	ch := s.ready[index(isNative)]

	// TODO: add conditiion if stage had been played
	if s.disabled || 0 == s.taskset.Len() {
		s.callNotifierChain(ctx)
		close(ch)
	}

	return ch
}

// Play perform tasks in the stage
func (s *Stage) play(ctx Context) error {

	isNative := ctx.Get("ISNATIVE").(bool)
	executed := &s.executed[index(isNative)]

	if atomic.LoadUint32(executed) == 1 {
		return nil
	}

	s.m.Lock()
	defer s.m.Unlock()
	defer func() {
		s.callNotifierChain(ctx)
		close(s.ready[index(isNative)])
	}()

	if s.disabled {
		return nil
	}

	// wait its dependent stages belong to another rubooks are finished
	for _, d := range s.depends {

		runbook := d.runbook
		stage := ""
		if i := strings.LastIndex(runbook, "@"); i >= 0 {
			runbook = d.runbook[:i]
			stage = d.runbook[i+1:]
		}
		select {
		case <-ctx.Ctx().Done():
			return ctx.Ctx().Err()
		case <-ctx.Wait(runbook, stage, isNative, d.notifier):
		}
	}

	if *executed == 0 {

		defer atomic.StoreUint32(executed, 1)

		if handled, err := s.rangeIn(ctx, s.name); handled || err != nil {
			return err
		}

		if err := s.taskset.play(ctx); err != nil {
			return err
		}
		return s.rangeOut(ctx, s.name)
	}
	return nil
}

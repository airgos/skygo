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

// Error used by Runbook
var (
	ErrUnknownTaskType = errors.New("Unkown Task Type")
	ErrUnknownTask     = errors.New("Unkown Task")
)

// Runbook consists of a series of stage and a independent taskset
type Runbook struct {
	head *list.List

	// inherits event listeners
	// it's for TaskForce
	listeners

	taskForce map[string]*TaskForce
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

	depends []string // dep format: runbookName[@stageName]
	ready   [2]chan struct{}

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

// String output information stages and independent tasks
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
func (rb *Runbook) runTaskForce(ctx context.Context, name string, w Waiter) error {

	tf, ok := rb.taskForce[name]
	if !ok {
		return fmt.Errorf("Runbook has no task force %s", name)
	}

	arg := FromContext(ctx)
	isNative := arg.Get("ISNATIVE").(bool)

	// wait its dependent stages belong to another rubooks are finished
	for _, d := range tf.depends {

		runbook := d
		stage := ""
		if i := strings.LastIndex(d, "@"); i >= 0 {
			runbook = d[:i]
			stage = d[i+1:]
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.Wait(runbook, stage, isNative):
		}
	}

	log.Trace("Run task force: %s", name)
	if handled, err := rb.rangeIn(name, arg); handled || err != nil {
		return err
	}

	// if taskset's dir is empty, try to use S
	if tf.taskset.Dir == "" {
		if dir, ok := arg.LookupVar("S"); ok {
			tf.taskset.Dir = dir
		}
	}

	// TaskForce only have one task
	if err := tf.taskset.runtask(ctx, 0); err != nil {
		return err
	}
	return rb.rangeOut(name, arg)
}

// Range iterates all stages and execute Play in the runbook
// Abort if any stage failed
func (rb *Runbook) Range(ctx context.Context, w Waiter) error {

	arg := FromContext(ctx)
	log.Trace("Range stages held by %s", arg.Owner)
	for stage := rb.Head(); stage != nil; stage = stage.Next() {
		if stage.taskset.Len() > 0 {

			err := stage.play(ctx, w)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Play run stage's tasks or the independent task
func (rb *Runbook) Play(ctx context.Context, name string, w Waiter) error {

	arg := FromContext(ctx)

	if s := rb.Stage(name); s != nil {
		if num := s.taskset.Len(); num > 0 {
			log.Trace("Play stage %s[tasks=%d] held by %s",
				name, num, arg.Owner)
			return s.play(ctx, w)
		}
		log.Warning("Stage %s held by %s has no tasks", name, arg.Owner)
		return nil
	}
	log.Trace("Run independent task %s held by %s", name, arg.Owner)
	return rb.runTaskForce(ctx, name, w)
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

// Dir specifies the working directory of the stage explicitly
func (s *Stage) Dir(dir string) *Stage {
	s.taskset.Dir = dir
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
func (s *Stage) Reset(ctx context.Context) {

	arg := FromContext(ctx)
	s.rangeReset(s.name, arg)

	s.m.Lock()
	defer s.m.Unlock()

	isNative := arg.Get("ISNATIVE").(bool)
	atomic.StoreUint32(&s.executed[index(isNative)], 0)
}

// AddDep add one dependent stage who are belong to another runbooks to current
// stage.  dep format: runbookName[@stageName]
func (s *Stage) AddDep(d string) *Stage {
	s.m.Lock()
	defer s.m.Unlock()

	s.depends = append(s.depends, d)
	return s
}

// Waiter is the interface to wait one dependent stage belong to another runbook finished
type Waiter interface {
	Wait(runbook, stage string, isNative bool) <-chan struct{}
}

func index(isNative bool) int {

	index := 0
	if isNative {
		index = 1
	}
	return index
}

// Wait return channel for waiting this stage is finished
func (s *Stage) Wait(isNative bool) <-chan struct{} {
	return s.ready[index(isNative)]
}

// Play perform tasks in the stage
func (s *Stage) play(ctx context.Context, w Waiter) error {

	arg := FromContext(ctx)
	isNative := arg.Get("ISNATIVE").(bool)
	executed := &s.executed[index(isNative)]

	if atomic.LoadUint32(executed) == 1 {
		return nil
	}

	s.m.Lock()
	defer s.m.Unlock()
	defer close(s.ready[index(isNative)])

	if s.disabled {
		return nil
	}

	// wait its dependent stages belong to another rubooks are finished
	for _, d := range s.depends {

		runbook := d
		stage := ""
		if i := strings.LastIndex(d, "@"); i >= 0 {
			runbook = d[:i]
			stage = d[i+1:]
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.Wait(runbook, stage, isNative):
		}
	}

	if *executed == 0 {

		defer atomic.StoreUint32(executed, 1)

		if handled, err := s.rangeIn(s.name, arg); handled || err != nil {
			return err
		}

		// if taskset's dir is empty, try to use S
		if s.taskset.Dir == "" {
			if dir, ok := arg.LookupVar("S"); ok {
				s.taskset.Dir = dir
			}
		}

		if err := s.taskset.play(ctx); err != nil {
			return err
		}
		return s.rangeOut(s.name, arg)
	}
	return nil
}

// Arg holds arguments for runbook
type Arg struct {
	// who own this, same as GetVar("PN")
	Owner string

	Private interface{} // private data

	// FilesPath is a collection of directory that's be used for locating local file
	FilesPath []string

	KV          // inherits KV, each runbook context has its own KV
	Kv KVGetter // extenral KV Getter

	// underline IO, call method Output() to get IO
	stdout, stderr io.Writer

	// help to wrap IO based on underline IO
	// example: use io.MultiWriter to duplicates its writes
	Writer func() (stdout, stderr io.Writer)

	m sync.Mutex
}

// Range iterates external and internal key-value data
func (arg *Arg) Range(f func(key, value string)) {
	arg.Kv.Range(f)
	arg.KV.Range(f)
}

// LookupVar retrieves the value of the variable named by the key.
// If the variable is present, value (which may be empty) is returned
// and the boolean is true. Otherwise the returned value will be empty
// and the boolean will be false.
func (arg *Arg) LookupVar(key string) (string, bool) {

	// get from external KV firstly
	if value, ok := arg.Kv.LookupVar(key); ok {
		return value, true
	}

	value, ok := arg.KV.LookupVar(key)
	return value, ok
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

// UnderOutput return underline IO stdout & stderr
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
func FromContext(ctx context.Context) *Arg {
	arg, ok := ctx.Value(argToken("arg")).(*Arg)
	if !ok {
		panic("Context don't bind Arg")
	}
	return arg
}

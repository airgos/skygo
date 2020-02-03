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
	"time"

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

	// retrieves standard context.Context
	Ctx() context.Context

	// retrieves timeout for stage, unit is second
	Timeout() int

	// check whether this stage identified by @name had been played
	Staged(name string) bool

	// Wait waits one dependent stage belong to another runbook finished
	// Wait should call Stage.Wait to add notifier to chain and wait stage done
	// nofier will be invoked when
	// 1. if stage had been executed, Stage's Wait iterats notifier chain
	// 2. upon stage is executed by Stage's Play
	Wait(ctx Context, runbook, stage string, notifier Notifer) <-chan struct{}

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

	// inherits notifier chain
	notifierChain

	taskForce map[string]*TaskForce
}

type stageDep struct {
	runbook  string //format: runbookName[@stageName]
	notifier Notifer
}

// Stage is member of Runbook, and hold a set of tasks with differnt weight.
// task with the lowest weight is executed first.
type Stage struct {
	name string
	help string

	taskset *TaskSet

	e       *list.Element
	runbook *Runbook

	m sync.Mutex

	disabled bool

	depends []stageDep

	// inherits notifier chain
	notifierChain
}

// NewRunbook create a Runbook
func NewRunbook() *Runbook {

	this := new(Runbook)
	this.head = list.New()
	this.taskForce = make(map[string]*TaskForce)
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
		case <-ctx.Wait(ctx, runbook, stage, nil):
		}
	}

	log.Trace("Run task force owned by %s: %s", ctx.Owner(), name)
	if err := rb.callNotifierChain(ctx, ENTER, name); err != nil {
		return err
	}

	if err := ctx.Acquire(); err != nil {
		return err
	}
	defer ctx.Release()

	// TaskForce only have one task
	if err := tf.taskset.runtask(ctx, 0); err != nil {
		return err
	}
	return rb.callNotifierChain(ctx, EXIT, name)
}

// Play run task force or iterates stages until stage @target
// if @target is emptry, it will iterates all stages
func (rb *Runbook) Play(ctx Context, target string) error {
	var err error

	if rb.HasTaskForce(target) {
		return rb.runTaskForce(ctx, target)
	}

	if target != "" && nil == rb.Stage(target) {
		return fmt.Errorf("%s has no stage or task force %s", ctx.Owner(), target)
	}

	log.Trace("Range stages held by %s", ctx.Owner())
	for stage := rb.Head(); stage != nil; stage = stage.Next() {

		if num := stage.taskset.Len(); num > 0 {

			log.Trace("Play stage %s[tasks=%d] held by %s",
				target, num, ctx.Owner())

			timeout := ctx.Timeout()
			stdCtx, cancel := context.WithTimeout(ctx.Ctx(), time.Duration(timeout)*time.Second)
			defer cancel()

			stage := stage // https://golang.org/doc/faq#closures_and_goroutines
			waitDone := make(chan struct{})
			go func() {

				err = stage.play(ctx)
				close(waitDone)
			}()

			select {
			case <-stdCtx.Done():

				select {
				case <-waitDone: // stage finished successfully
				default:
					if stdCtx.Err() == context.DeadlineExceeded {

						return fmt.Errorf("Runbook expire on %s@%s over %d seconds",
							ctx.Owner(), stage.name, timeout)
					}
				}
			case <-waitDone:
				if err != nil {
					return err
				}
			}

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

	// init notifier chain for stage
	stage.notifierChain.init()

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

	s.callNotifierChain(ctx, RESET, s.name)
}

// AddDep add one dependent stage who are belong to another runbooks to current stage.
// format of parameter @d: runbookName[@stageName]
func (s *Stage) AddDep(d string,
	notifier func(ctx Context, stage string) error) *Stage {

	s.m.Lock()
	defer s.m.Unlock()

	s.depends = append(s.depends, stageDep{
		runbook:  d,
		notifier: Notifer(notifier),
	})

	return s
}

// Wait return channel for waiting this stage is finished
// nofier will be invoked when
// 1. stage had been executed. and iterats notifier chain here
// 2. upon stage is executed by Play
func (s *Stage) Wait(ctx Context, notifier Notifer) <-chan struct{} {

	if notifier != nil {
		s.RegisterNotifier(notifier, EXIT)
	}

	s.m.Lock()
	defer s.m.Unlock()

	ch := make(chan struct{})

	staged := ctx.Staged(s.name)
	if staged || s.disabled || 0 == s.taskset.Len() {
		if err := s.callNotifierChain(ctx, EXIT, s.name); err != nil {
			log.Error("Failed to iterate %s@%s's notifier chain since %s", ctx.Owner(), s.name, err)
		}
		close(ch)
	}

	if !staged {

		s.registerNotifierBack(Notifer(func(ctx Context, name string) error {
			close(ch)
			return nil
		}), EXIT)
	}

	return ch
}

// Play perform tasks in the stage
func (s *Stage) play(ctx Context) error {

	s.m.Lock()
	defer s.m.Unlock()

	if err := s.callNotifierChain(ctx, ENTER, s.name); err != nil {
		return err
	}

	if ctx.Staged(s.name) || s.disabled {
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
		case <-ctx.Wait(ctx, runbook, stage, d.notifier):
		}
	}

	if err := ctx.Acquire(); err != nil {
		return err
	}
	defer ctx.Release()

	if err := s.taskset.play(ctx); err != nil {
		return err
	}

	return s.callNotifierChain(ctx, EXIT, s.name)
}

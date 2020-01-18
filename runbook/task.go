// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runbook

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"skygo/utils/log"
)

// taskCmd represent shell script
type taskCmd struct {
	script  string // script file name or script string
	routine string // entry routine of script
}

// TaskGoFunc prototype
// dir is working directory
type TaskGoFunc func(ctx Context, dir string) error

// taskGo represent golang func task
type taskGo struct {
	f TaskGoFunc
}

// TaskSet represent a collection of task
// It supports two kind of task: taskGo or taskCmd
type TaskSet struct {
	set   map[interface{}]interface{}
	owner string //optional. who own this TaskSet
	Dir   string //optional. working directory
}

// newTaskSet create taskset
func newTaskSet() *TaskSet {
	t := new(TaskSet)
	t.set = make(map[interface{}]interface{})
	return t
}

// Len get the number of tasks
func (t *TaskSet) Len() int {
	return len(t.set)
}

// Add push one task to taskset
// It supports two kind of task: TaskGoFunc & script. script is a script file name or string
// if it's a script file, task runner will try to find it under FilesPath
func (t *TaskSet) Add(key interface{}, task interface{}) {

	v := task
	if _, ok := t.set[key]; ok {
		panic(fmt.Sprintf("Task %v had been owned by %s\n", key, t.owner))
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Print(r)
			os.Exit(2)
		}
	}()

	switch kind := task.(type) {

	case string:
		routine := t.owner
		if name, ok := key.(string); ok {
			routine = name
		}
		v = taskCmd{routine: routine, script: kind}

	case func(Context, string) error:
		v = taskGo{f: TaskGoFunc(kind)}

	case TaskGoFunc:
		v = taskGo{f: kind}

	default:
		b := strings.Builder{}
		b.WriteString(fmt.Sprintf("Unknown task type: %T\n\n", task))

		pc, file, line, _ := runtime.Caller(1)
		details := runtime.FuncForPC(pc)
		b.WriteString(fmt.Sprintf("%s:%d\n", file, line))
		b.WriteString(fmt.Sprintf("\t%s\n", details.Name()))

		pc, file, line, _ = runtime.Caller(2)
		details = runtime.FuncForPC(pc)
		b.WriteString(fmt.Sprintf("%s:%d\n", file, line))
		b.WriteString(fmt.Sprintf("\t%s\n", details.Name()))
		panic(b.String())
	}

	t.set[key] = v
}

// Del delete task
func (t *TaskSet) Del(key interface{}) {
	delete(t.set, key)
}

// play run all tasks by order of Sort.Ints(weight)
func (t *TaskSet) play(ctx Context) error {

	// sort weight
	weight := make([]int, 0, len(t.set))
	name := make([]string, 0, len(t.set))
	for k := range t.set {

		if v, ok := k.(int); ok {
			weight = append(weight, v)
		} else {
			name = append(name, k.(string))
		}
	}
	sort.Ints(weight)

	for _, w := range weight {
		if err := t.runtask(ctx, w); err != nil {
			return err
		}
	}
	for _, k := range name {
		if err := t.runtask(ctx, k); err != nil {
			return err
		}
	}
	return nil
}

func (t *TaskSet) runtask(ctx Context, key interface{}) (e error) {

	switch kind := t.set[key].(type) {
	default:
		// fmt.Printf("%T", kind)
		e = ErrUnknownTaskType
	case taskGo:
		e = kind.f(ctx, t.Dir)
	case taskCmd:
		e = kind.run(ctx, t.Dir)
	}
	return
}

// run the taskCmd, before run, it does:
// Locate tc.name under runtime GetFilePath(), if found, it's script file, else it's script string
// If script have function @routine, append routine name
func (tc *taskCmd) run(ctx Context, dir string) error {

	var r io.Reader
	routine := tc.routine

	// regular expression used to match shell function name
	exp := regexp.MustCompile(fmt.Sprintf(` *%s *\( *\)`, tc.routine))

	if len(strings.Split(tc.script, "\n")) == 1 {
		log.Trace("Try to find script under FilesPath")
		for _, d := range ctx.FilesPath() {
			path := filepath.Join(d, tc.script)
			if info, err := os.Stat(path); err == nil &&
				info.Mode().IsRegular() {

				b, _ := ioutil.ReadFile(path)
				if !exp.Match(b) {
					routine = ""
				}

				r, _ = os.Open(path)
				break
			}
		}
	}

	//script string
	if r == nil {
		if !exp.MatchString(tc.script) {
			routine = ""
		}
		r = strings.NewReader(tc.script)
	}

	command := NewCommand(ctx, "/bin/bash")
	if routine != "" {
		command.Cmd.Stdin = io.MultiReader(r, strings.NewReader(tc.routine))
	} else {
		command.Cmd.Stdin = r
	}
	if dir != "" {
		command.Cmd.Dir = dir
	}

	return command.Run(ctx, tc.routine)
}

// TaskForce is just a wrapper of TaskSet with dependency
type TaskForce struct {
	taskset *TaskSet
	summary string
	depends []string // dep format: runbookName[@stageName]
}

func newTaskForce(name, summry string) *TaskForce {
	tf := new(TaskForce)
	tf.taskset = newTaskSet()
	tf.summary = summry
	return tf
}

// setTask assign one task to TaskForce
// It supports two kind of task: TaskGoFunc & script. script is a script file
// name or string. if it's a script file, task runner will try to find it under
// FilesPath
func (tf *TaskForce) setTask(task interface{}) *TaskForce {
	tf.taskset.Add(0, task)
	return tf
}

// AddDep add one dependent stage who are belong to another runbooks to current
// TaskForce.  dep format: runbookName[@stageName]
func (tf *TaskForce) AddDep(d string) *TaskForce {
	tf.depends = append(tf.depends, d)
	return tf
}

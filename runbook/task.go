// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runbook

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"merge/log"
)

// taskCmd represent shell script
type taskCmd struct {
	script  string // script file name or script string
	routine string // entry routine of script
	summary string // description summary
}

// TaskGoFunc prototype
// dir is working directory
type TaskGoFunc func(ctx context.Context, dir string) error

// taskGo represent golang func task
type taskGo struct {
	f       TaskGoFunc
	summary string // description summary
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

// Has return whether TaskSet has task
func (t *TaskSet) Has(name string) bool {
	_, ok := t.set[name]
	return ok
}

// Add push one task to taskset. Return ErrTaskAdded if key exists
// It supports two kind of task: TaskGoFunc & script. script is a script file name or string
// if it's a script file, task runner will try to find it under FilesPath
func (t *TaskSet) Add(key interface{}, task interface{}, summary string) error {

	v := task
	if _, ok := t.set[key]; ok {
		log.Error("Task %v had been owned by %s\n", key, t.owner)
		return ErrTaskAdded
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
		v = taskCmd{routine: routine, script: kind, summary: summary}

	case func(context.Context, string) error:
		v = taskGo{f: TaskGoFunc(kind), summary: summary}

	case TaskGoFunc:
		v = taskGo{f: kind, summary: summary}

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
	return nil
}

// Del delete task
func (t *TaskSet) Del(key interface{}) {
	delete(t.set, key)
}

// play run all tasks by order of Sort.Ints(weight)
func (t *TaskSet) play(ctx context.Context) error {

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

func (t *TaskSet) runtask(ctx context.Context, key interface{}) (e error) {

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
func (tc *taskCmd) run(ctx context.Context, dir string) error {

	var r io.Reader
	routine := tc.routine
	arg := FromContext(ctx)

	// regular expression used to match shell function name
	exp := regexp.MustCompile(fmt.Sprintf(` *%s *\( *\)`, tc.routine))

	if len(strings.Split(tc.script, "\n")) == 1 {
		log.Trace("Try to find script under FilesPath")
		for _, d := range arg.FilesPath {
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

	return command.Run(tc.routine)
}

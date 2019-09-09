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
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"merge/log"
)

// TaskCmd represent command name with enter routine
type TaskCmd struct {
	name    string // file name, script string
	routine string //optional
}

// TaskSet represent a collection of task
// It supports two kind of task: golang func or TaskCmd
type TaskSet struct {
	set     map[interface{}]interface{}
	routine string
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

// Add task. Return ErrTaskAdded if key was set
// TODO: return TaskCmd ?
func (t *TaskSet) Add(key interface{}, task interface{}) (*TaskSet, error) {

	v := task
	if _, ok := t.set[key]; ok {

		log.Error("Task(%s) added: %v\n", t.routine, task)
		return t, ErrTaskAdded
	}

	if name, ok1 := task.(string); ok1 {
		if routine, ok2 := key.(string); ok2 {
			v = TaskCmd{routine: routine, name: name}
		} else {
			v = TaskCmd{routine: t.routine, name: name}
		}
	}
	t.set[key] = v
	return t, nil
}

// Del delete task
func (t *TaskSet) Del(key interface{}) {
	delete(t.set, key)
}

// Run specific task
func (t *TaskSet) Run(ctx context.Context, key string) error {

	if task, ok := t.set[key]; ok {
		if err := t.runtask(ctx, task); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("Unknown task %s", key)
	}
	return nil
}

// Play run all task by order of Sort.Ints(weight)
func (t *TaskSet) Play(ctx context.Context) error {

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
		if err := t.runtask(ctx, t.set[w]); err != nil {
			return err
		}
	}
	for _, k := range name {
		if err := t.runtask(ctx, t.set[k]); err != nil {
			return err
		}
	}
	return nil
}

func (t *TaskSet) runtask(ctx context.Context, task interface{}) (e error) {

	switch kind := task.(type) {
	default:
		// fmt.Printf("%T", kind)
		e = ErrUnknownTaskType
	case func(context.Context) error:
		e = kind(ctx)
	case TaskCmd:
		e = kind.Run(ctx)
	}
	return
}

// Run the TaskCmd, before run, it does:
// Locate tc.name under runtime GetFilePath(), if found, it's script file, else it's script string
// If script have function @routine, append routine name
func (tc *TaskCmd) Run(ctx context.Context) error {

	var r io.Reader
	routine := tc.routine
	arg, _ := FromContext(ctx)

	// regular expression used to match shell function name
	exp := regexp.MustCompile(fmt.Sprintf(` *%s *\( *\)`, tc.routine))

	for _, d := range arg.Direnv.FilePath() {
		path := filepath.Join(d, tc.name)
		if _, err := os.Stat(path); err == nil {

			b, _ := ioutil.ReadFile(path)
			if !exp.Match(b) {
				routine = ""
			}

			r, _ = os.Open(path)
			break
		}
	}
	if r == nil {
		if !exp.MatchString(tc.name) {
			routine = ""
		}
		r = strings.NewReader(tc.name)
	}

	cmd := exec.CommandContext(ctx, "/bin/bash")
	cmd.Stdout, cmd.Stderr = arg.Output()
	cmd.Dir = arg.Direnv.SrcPath()

	if routine != "" {

		cmd.Stdin = io.MultiReader(r, strings.NewReader(tc.routine))
	} else {
		cmd.Stdin = r

	}
	cmd.Env = append(cmd.Env, arg.Direnv.Environ()...)

	if e := cmd.Run(); e != nil {
		return fmt.Errorf("Runbook: %s", e)
	}
	return nil
}

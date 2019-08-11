package runbook

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Runtime implement runtime method
type Runtime interface {
	SrcPath() string    // where to run
	FilePath() []string // where to find script file
	Output() (stdout, stderr io.Writer)
	Environ() []string
}

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

// Add task. Return ErrTaskAdded if key was set
// TODO: return TaskCmd ?
func (t *TaskSet) Add(key interface{}, task interface{}) (*TaskSet, error) {

	v := task
	if _, ok := t.set[key]; ok {

		// TODO: use log
		fmt.Printf("Task(%s) added: %v\n", t.routine, task)
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
func (t *TaskSet) Run(key string, r Runtime) error {

	if task, ok := t.set[key]; ok {
		if err := t.runtask(task, r); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("No task %s", key)
	}
	return nil
}

// Play run all task by Sort.Ints
func (t *TaskSet) Play(r Runtime) error {

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
		if err := t.runtask(t.set[w], r); err != nil {
			return err
		}
	}
	for _, k := range name {
		if err := t.runtask(t.set[k], r); err != nil {
			return err
		}
	}
	return nil
}

func (t *TaskSet) runtask(task interface{}, r Runtime) (e error) {

	switch kind := task.(type) {
	default:
		// fmt.Printf("%T", kind)
		e = ErrUnknownTaskType
	case func() error:
		e = kind()
	case TaskCmd:
		e = kind.Run(r)
	}
	return
}

// Run the TaskCmd, before run, it does:
// Locate tc.name under runtime GetFilePath(), if found, it's script file, else it's script string
// If script have function @routine, append routine name
// TODO:
// cmd timeout
func (tc *TaskCmd) Run(tr Runtime, kv ...string) error {

	var r io.Reader
	routine := tc.routine

	// regular expression used to match shell function name
	exp := regexp.MustCompile(fmt.Sprintf(` *%s *\( *\)`, tc.routine))

	for _, d := range tr.FilePath() {
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

	cmd := exec.Command("/bin/bash")
	cmd.Dir = tr.SrcPath()
	cmd.Stdout, cmd.Stderr = tr.Output()

	if routine != "" {

		cmd.Stdin = io.MultiReader(r, strings.NewReader(tc.routine))
	} else {
		cmd.Stdin = r

	}
	cmd.Env = append(cmd.Env, tr.Environ()...)
	cmd.Env = append(cmd.Env, kv...)

	if e := cmd.Run(); e != nil {
		return fmt.Errorf("Runbook: %s", e)
	}
	return nil
}

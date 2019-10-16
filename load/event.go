// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package load

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"merge/carton"
	"merge/log"
	"merge/runbook"
)

func addEventListener(rb *runbook.Runbook) {

	for stage := rb.Head(); stage != nil; stage = stage.Next() {

		stage.PushInOut(logfileEnter, logfileExit)
		stage.PushReset(stageReset)

		// always play stage FETCH, then FETCH has chance to detect code change
		if stage.Name() != carton.FETCH {
			stage.PushInOut(stageStatus, stageSetDone)
		} else {
			// before souce code is fetched, generally var S is empty.
			// call tryToSetVarS upon quiting fetch stage to update var S
			stage.PushInOut(nil, tryToSetVarS)
		}

	}
	rb.PushInOut(cleanTask, nil)
}

func logfileExit(name string, arg *runbook.Arg, x interface{}) error {
	file := x.(*os.File)
	file.Close()
	return nil
}

func logfileEnter(stage string, arg *runbook.Arg) (bool, interface{}, error) {

	logfile := filepath.Join(arg.Vars["T"], stage+".log")
	file, err := os.Create(logfile)
	if err != nil {
		return false, nil, fmt.Errorf("Failed to create %s", logfile)
	}

	arg.Writer = func() (io.Writer, io.Writer) {
		stdout, stderr := arg.UnderOutput()
		if stdout != nil {
			stdout = io.MultiWriter(stdout, file)
		} else {
			stdout = file
		}
		if stderr != nil {
			stderr = io.MultiWriter(stderr, file)
		} else {
			stderr = file
		}
		return stdout, stderr
	}
	return false, file, nil
}

func stageSetDone(stage string, arg *runbook.Arg, x interface{}) error {

	done := x.(string)
	os.Create(done)
	return nil
}

func stageStatus(stage string, arg *runbook.Arg) (bool, interface{}, error) {

	done := filepath.Join(arg.Vars["T"], stage+".done")

	if _, err := os.Stat(done); err == nil {
		log.Trace("%s was executed last time, skip it!", stage)
		return true, nil, nil
	}
	return false, done, nil
}

func stageReset(stage string, arg *runbook.Arg) error {

	done := filepath.Join(arg.Vars["T"], stage+".done")
	os.Remove(done)
	return nil
}

func cleanTask(task string, arg *runbook.Arg) (bool, interface{}, error) {
	if task == "clean" {
		os.RemoveAll(arg.Vars["T"])

		// only run clean task if S does exist
		dir := arg.Vars["S"]
		if dir == "" {
			return true, nil, nil
		}

		if _, err := os.Stat(dir); err != nil {
			return true, nil, nil
		}
	}
	return false, nil, nil
}

func tryToSetVarS(stage string, arg *runbook.Arg, x interface{}) error {
	if arg.Vars["S"] != "" {
		return nil
	}

	c, _, _, _ := carton.Find(arg.Owner)
	if dir := c.SrcDir(arg.Wd); dir == "" {
		return fmt.Errorf("Failed to find SrcDir automatically! Please set it explicitily by SetSrcDir.")
	} else {
		arg.Vars["S"] = dir
	}
	return nil
}

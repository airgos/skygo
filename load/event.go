// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package load

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"skygo/carton"
	"skygo/runbook"
	"skygo/utils"
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

	logfile := filepath.Join(arg.GetVar("T"), stage+".log")
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

	markStagePlayed(stage, arg.GetVar("T"), true)
	return nil
}

func stageStatus(stage string, arg *runbook.Arg) (bool, interface{}, error) {

	return isStagePlayed(stage, arg.GetVar("T")), nil, nil
}

func stageReset(stage string, arg *runbook.Arg) error {

	markStagePlayed(stage, arg.GetVar("T"), false)
	return nil
}

func cleanTask(task string, arg *runbook.Arg) (bool, interface{}, error) {
	if task == "clean" {
		os.RemoveAll(arg.GetVar("T"))

		// only run clean task if S does exist
		dir, ok := arg.LookupVar("S")
		if !ok {
			return true, nil, nil
		}

		if !utils.IsExist(dir) {
			return true, nil, nil
		}
	}
	return false, nil, nil
}

func tryToSetVarS(stage string, arg *runbook.Arg, x interface{}) error {
	if _, ok := arg.LookupVar("S"); ok {
		return nil
	}

	c := arg.Private.(carton.Builder)
	if dir := c.SrcDir(arg.GetVar("WORKDIR")); dir == "" {
		return fmt.Errorf("Failed to find SrcDir automatically! Please set it explicitily by SetSrcDir.")
	} else {
		arg.SetKv("S", dir)
	}
	return nil
}

// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package load

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"merge/runbook"
)

func addEventListener(rb *runbook.Runbook) {

	for stage := rb.Head(); stage != nil; stage = stage.Next() {
		stage.PushInOut(logfileEnter, logfileExit)
	}
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

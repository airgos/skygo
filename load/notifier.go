// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package load

import (
	"fmt"
	"os"
	"path/filepath"

	"skygo/carton"
	"skygo/runbook"
	"skygo/utils"
)

func registerNotifier(rb *runbook.Runbook) {

	for stage := rb.Head(); stage != nil; stage = stage.Next() {

		stage.RegisterNotifier(logfileEnter, runbook.ENTER)
		stage.RegisterNotifier(logfileExit, runbook.EXIT)
		stage.RegisterNotifier(stageReset, runbook.RESET)

		// always play stage FETCH, then FETCH has chance to detect code change
		if stage.Name() != carton.FETCH {
			stage.RegisterNotifier(stageSetDone, runbook.EXIT)
		} else {
			// before souce code is fetched, generally var S is empty.
			// call tryToSetVarS upon quiting fetch stage to update var S
			stage.RegisterNotifier(tryToSetVarS, runbook.EXIT)
		}

	}
	rb.RegisterNotifier(cleanTask, runbook.ENTER)
}

func logfileExit(ctx runbook.Context, stage string) error {

	carton := getCartonFromCtx(ctx)
	load := getLoadFromCtx(ctx)
	s := load.getStage(carton.Provider(), stage, ctx.Get("ISNATIVE").(bool))
	stdout, stderr := s.getIO()

	if stdout == stderr && stdout != nil {
		stdout.Close()
	} else {
		if stdout != nil {
			stdout.Close()

		}
		if stderr != nil {
			stderr.Close()
		}
	}

	return nil
}

func logfileEnter(ctx runbook.Context, stage string) error {

	logfile := filepath.Join(ctx.GetStr("T"), stage+".log")
	file, err := os.Create(logfile)
	if err != nil {
		return fmt.Errorf("Failed to create %s", logfile)
	}

	carton := getCartonFromCtx(ctx)
	load := getLoadFromCtx(ctx)
	s := load.getStage(carton.Provider(), stage, ctx.Get("ISNATIVE").(bool))
	s.setIO(file, file)

	setStageToCtx(ctx, stage)

	return nil
}

func stageSetDone(ctx runbook.Context, stage string) error {

	markStagePlayed(ctx.Owner(), stage, ctx.GetStr("T"), true)
	load := getLoadFromCtx(ctx)
	load.markStageDone(ctx.Owner(), stage, ctx.Get("ISNATIVE").(bool))
	return nil
}

func stageReset(ctx runbook.Context, stage string) error {

	markStagePlayed(ctx.Owner(), stage, ctx.GetStr("T"), false)
	return nil
}

func cleanTask(ctx runbook.Context, task string) error {
	if task == "clean" {
		os.RemoveAll(ctx.GetStr("T"))

		// only run clean task if S does exist
		_, dir := ctx.Dir()
		if dir == "" {
			return fmt.Errorf("Not set environment variable S or B")
		}

		if !utils.IsExist(dir) {
			return fmt.Errorf("Dir %s is not found!", dir)
		}
	}
	return nil
}

func tryToSetVarS(ctx runbook.Context, stage string) error {
	if nil != ctx.Get("S") {
		return nil
	}

	if s, _ := ctx.Dir(); s == "" {
		return fmt.Errorf("Failed to find SrcDir automatically! Please set it explicitily by SetSrcDir.")
	} else {
		ctx.Set("S", s)
	}
	return nil
}

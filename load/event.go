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

func logfileExit(ctx runbook.Context, name string, x interface{}) error {
	file := x.(*os.File)
	file.Close()
	return nil
}

func logfileEnter(ctx runbook.Context, stage string) (bool, interface{}, error) {

	logfile := filepath.Join(ctx.GetStr("T"), stage+".log")
	file, err := os.Create(logfile)
	if err != nil {
		return false, nil, fmt.Errorf("Failed to create %s", logfile)
	}

	carton := getCartonFromCtx(ctx)
	load := getLoadFromCtx(ctx)
	s := load.getStage(carton.Provider(), stage, ctx.Get("ISNATIVE").(bool))
	s.setIO(file, file)

	setStageToCtx(ctx, stage)

	return false, file, nil
}

func stageSetDone(ctx runbook.Context, stage string, x interface{}) error {

	markStagePlayed(ctx.Owner(), stage, ctx.GetStr("T"), true)
	load := getLoadFromCtx(ctx)
	load.markStageDone(ctx.Owner(), stage, ctx.Get("ISNATIVE").(bool))
	return nil
}

func stageStatus(ctx runbook.Context, stage string) (bool, interface{}, error) {

	return isStagePlayed(ctx.Owner(), stage, ctx.GetStr("T")), nil, nil
}

func stageReset(ctx runbook.Context, stage string) error {

	markStagePlayed(ctx.Owner(), stage, ctx.GetStr("T"), false)
	return nil
}

func cleanTask(ctx runbook.Context, task string) (bool, interface{}, error) {
	if task == "clean" {
		os.RemoveAll(ctx.GetStr("T"))

		// only run clean task if S does exist
		dir := ctx.Get("S")
		if dir == nil {
			return true, nil, nil
		}

		if !utils.IsExist(dir.(string)) {
			return true, nil, nil
		}
	}
	return false, nil, nil
}

func tryToSetVarS(ctx runbook.Context, stage string, x interface{}) error {
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

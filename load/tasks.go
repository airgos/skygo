// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package load

import (
	"fmt"
	"os"

	"skygo/carton"
	"skygo/runbook"
	"skygo/utils/log"
)

func cleanall(ctx runbook.Context) error {

	wd := ctx.GetStr("WORKDIR")

	os.RemoveAll(wd)
	log.Trace("Remove working dir %s", wd)
	return nil
}

func printenv(ctx runbook.Context) error {

	fmt.Println()
	ctx.Range(func(k, v string) {
		// _* is internal key, ignore it
		if k[0] != '_' {
			fmt.Printf("%12s:\t%s\n", k, v)
		}
	})
	return nil
}

func cleanstate(ctx runbook.Context) error {

	cleanstate1(getCartonFromCtx(ctx), "", ctx.GetStr("T"))
	return nil
}

func cleanstate1(c carton.Builder, target, tempDir string) {

	carton := c.Provider()
	if target != "" {
		markStagePlayed(carton, target, tempDir, false)
	} else {
		for stage := c.Runbook().Head(); stage != nil; stage = stage.Next() {
			markStagePlayed(carton, stage.Name(), tempDir, false)
		}
	}
}

// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package load

import (
	"context"
	"fmt"
	"os"

	"skygo/carton"
	"skygo/runbook"
	"skygo/utils/log"
)

func cleanall(ctx context.Context, dir string) error {

	arg := runbook.FromContext(ctx)
	wd := arg.GetStr("WORKDIR")

	os.RemoveAll(wd)
	log.Trace("Remove working dir %s", wd)
	return nil
}

func printenv(ctx context.Context, dir string) error {

	arg := runbook.FromContext(ctx)
	fmt.Println()
	arg.Range(func(k, v string) {
		fmt.Printf("%12s:\t%s\n", k, v)
	})
	return nil
}

func cleanstate(ctx context.Context, dir string) error {

	arg := runbook.FromContext(ctx)
	c := arg.Private.(carton.Builder)
	cleanstate1(c, "", arg.GetStr("T"))
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

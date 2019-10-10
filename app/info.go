// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"context"
	"flag"
	"fmt"
	"merge/carton"
	"merge/load"
)

type info struct {
}

func (*info) Name() string         { return "info" }
func (*info) UsageLine() string    { return "<carton name>" }
func (*info) Summary() string      { return "show information of carton" }
func (*info) Help(f *flag.FlagSet) {}

func (*info) Run(ctx context.Context, args ...string) error {
	if len(args) == 0 {
		return commandLineErrorf("carton name must be supplied")
	}

	c, virtual, e := carton.Find(args[0])
	if e != nil {
		return fmt.Errorf("carton %s is %v", args[0], e)
	}

	if virtual {
		fmt.Printf("%s --> %s\n\n", args[0], c.Provider())
	}
	load.SetupRunbook(c.Runbook())

	show(c)

	return nil
}

func show(h carton.Builder) {

	// TODO:
	// indicates whether it is installed
	// highlight selected version
	fmt.Printf("%s", h.Provider())
	versions := h.Resource().Versions()
	if len(versions) > 0 {
		fmt.Printf(": %s", versions[0])
		for _, ver := range versions[1:] {
			fmt.Printf(", %s", ver)
		}
	}
	fmt.Println()
	fmt.Println(h)

	// print dependencies
	builds := h.BuildDepends()
	depends := h.Depends()
	if len(builds) > 0 || len(depends) > 0 {
		fmt.Println("==> Dependencies")
		if len(builds) > 0 {

			fmt.Printf("Build: %s", builds[0])
			for _, d := range builds[1:] {
				fmt.Printf(", %s", d)
			}
			fmt.Println()
		}

		if len(depends) > 0 {

			fmt.Printf("Required: %s", depends[0])
			for _, d := range depends[1:] {
				fmt.Printf(", %s", d)
			}
			fmt.Println()
		}
	}

	fmt.Println("==> Path")
	wd := load.WorkDir(h, false)
	fmt.Println("WORKDIR:", wd)
	fmt.Println("SRCDIR: ", h.SrcPath(wd))

	filespath := h.FilesPath()
	fmt.Println("FilesPath:", filespath[0])
	for _, p := range filespath[1:] {
		fmt.Println("     ", p)
	}

	fmt.Println("==> Runbook")
	stage, tasknum, taskname := h.Runbook().RunbookInfo()
	fmt.Printf("Stage: %s[%d]", stage[0], tasknum[0])
	for i := 1; i < len(stage); i++ {
		fmt.Printf(" --> %s[%d]", stage[i], tasknum[i])
	}
	if len(taskname) > 0 {
		fmt.Printf("\nTasks: %s", taskname[0])
		for _, n := range taskname[1:] {
			fmt.Printf(" %s", n)
		}
	}
}

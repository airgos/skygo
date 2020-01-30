// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"context"
	"flag"
	"fmt"

	"skygo/carton"
	"skygo/load"
	"skygo/runbook"
)

type info struct {
	name string //top cmd name
}

func (*info) Name() string         { return "info" }
func (*info) UsageLine() string    { return "<carton name>" }
func (*info) Summary() string      { return "show information of carton" }
func (*info) Help(f *flag.FlagSet) {}

func (i *info) Run(ctx context.Context, args ...string) error {
	if len(args) == 0 {
		return commandLineErrorf("carton name must be supplied")
	}

	l, _ := load.NewLoad(ctx, i.name)
	return l.Info(args[0], func(ctx runbook.Context, carton carton.Builder, virtual bool) {

		if virtual {
			fmt.Printf("%s --> %s\n\n", args[0], carton.Provider())
		}
		show(ctx, carton)

	})
}

func show(ctx runbook.Context, c carton.Builder) {

	// TODO:
	// indicates whether it is installed
	// highlight selected version
	fmt.Printf("%s", c.Provider())
	versions := c.Resource().Versions()
	if len(versions) > 0 {
		fmt.Printf(": %s", versions[0])
		for _, ver := range versions[1:] {
			fmt.Printf(", %s", ver)
		}
	}
	fmt.Println()
	fmt.Println(c)

	// print dependencies
	builds := c.BuildDepends()
	depends := c.Depends()
	if len(builds) > 0 || len(depends) > 0 {
		fmt.Println("==> Dependencies")
		if len(builds) > 0 {

			fmt.Printf("   Build: %s", builds[0])
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
		fmt.Println()
	}

	fmt.Println("==> Path")
	wd := ctx.GetStr("WORKDIR")
	fmt.Println("  WORKDIR:", wd)
	fmt.Println("   SRCDIR:", c.SrcDir(wd))

	filespath := c.FilesPath()
	fmt.Println("FilesPath:", filespath[0])
	for _, p := range filespath[1:] {
		fmt.Println("          ", p)
	}

	fmt.Printf("\n==> Runbook")
	fmt.Println(c.Runbook())
}

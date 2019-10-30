// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"merge/load"
	"merge/log"
)

type build struct {
	name    string //top cmd name
	NoDeps  bool   `flag:"nodeps" help:"don't check dependency"`
	Loaders int    `flag:"loaders" help:"set the number of jobs to build cartons"`
	Force   bool   `flag:"force" help:"force to run"`
}

func (*build) Name() string      { return "carton" }
func (*build) UsageLine() string { return "<carton name[@target]>" }
func (*build) Summary() string {
	return `build carton
target is stage name or independent task name
info command can show stage & task information
	`
}

func (*build) Help(f *flag.FlagSet) {

	fmt.Fprintf(f.Output(), "\ncarton flags are:\n")
	f.PrintDefaults()
}

func (b *build) Run(ctx context.Context, args ...string) error {
	if len(args) == 0 {
		return commandLineErrorf("carton name must be supplied")
	}

	panes := tmuxPanes(ctx)
	if num := len(panes); num > 0 {
		b.Loaders = num
	}

	log.Trace("MaxLoaders is set to %d\n", b.Loaders)
	l := load.NewLoad(ctx, b.name, b.Loaders)
	for i, pane := range panes {
		if file, err := os.OpenFile(pane, os.O_RDWR, 0766); err == nil {
			l.SetOutput(i, file, file)
		} else {
			log.Warning("Failed to open tmux pane %s. Error: %s\n", pane, err)
		}
	}

	carton := args[0]
	target := ""
	if i := strings.LastIndex(args[0], "@"); i >= 0 {
		carton = args[0][:i]
		target = args[0][i+1:]
	}
	return l.Run(carton, target, b.NoDeps, b.Force)
}

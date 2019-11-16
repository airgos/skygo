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

	"skygo/load"
	"skygo/log"
)

type build struct {
	name    string //top cmd name
	NoDeps  bool   `flag:"nodeps" help:"don't check dependency"`
	Loaders int    `flag:"loaders" help:"set the number of jobs to build cartons"`
	Force   bool   `flag:"force" help:"force to run"`
	Verbose bool   `flag:"v" help:"verbose output. available when no tmux panes"`
}

func (*build) Name() string    { return "carton" }
func (*build) Summary() string { return "build carton" }
func (b *build) UsageLine() string {
	return fmt.Sprintf(`<carton name[@target]>

target should be stage name or independent task name.
command info show what stages and independent tasks carton has.

example:

$%s carton busybox@fetch
`, b.name)
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
	numPanes := len(panes)
	if numPanes > 0 {
		if b.Loaders == 0 {
			b.Loaders = numPanes
		} else { // cmd line assign Loaders
			if numPanes < b.Loaders {
				b.Loaders = numPanes
			}
		}
	}

	l, loaders := load.NewLoad(ctx, b.name, b.Loaders)
	if numPanes > 0 {
		for i := 0; i < b.Loaders; i++ {
			pane := panes[i]
			if file, err := os.OpenFile(pane, os.O_RDWR, 0766); err == nil {
				l.SetOutput(i, file, file)
			} else {
				log.Warning("Failed to open tmux pane %s. Error: %s\n", pane, err)
			}
		}
	} else if b.Verbose {
		for i := 0; i < loaders; i++ {
			l.SetOutput(i, os.Stdout, os.Stderr)
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

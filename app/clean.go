// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"context"
	"flag"
	"fmt"
	"merge/load"
	"merge/runbook"
)

type clean struct {
	Force bool `flag:"force" help:"remove working path"`
}

func (*clean) Name() string      { return "clean" }
func (*clean) UsageLine() string { return "<carton name>" }
func (*clean) Summary() string   { return "clean carton" }

func (*clean) Help(f *flag.FlagSet) {

	fmt.Fprintf(f.Output(), "\ncarton flags are:\n")
	f.PrintDefaults()
}

func (c *clean) Run(ctx context.Context, args ...string) error {
	if len(args) == 0 {
		return commandLineErrorf("carton name must be supplied")
	}

	err := load.NewLoad(1).Clean(ctx, args[0], c.Force)
	if err == runbook.ErrUnknownTask {
		return fmt.Errorf("carton %s has no task clean, try to add flag -force", args[0])
	}
	return err
}

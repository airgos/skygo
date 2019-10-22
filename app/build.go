// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"context"
	"flag"
	"fmt"

	"merge/load"
)

type build struct {
	name    string //top cmd name
	NoDeps  bool   `flag:"nodeps" help:"don't check dependency"`
	Target  string `flag:"play" help:"one indenpent task or stage name"`
	Loaders int    `flag:"loaders" help:"set the number of jobs to build cartons"`
}

func (*build) Name() string      { return "carton" }
func (*build) UsageLine() string { return "<carton name>" }
func (*build) Summary() string   { return "build carton" }

func (*build) Help(f *flag.FlagSet) {

	fmt.Fprintf(f.Output(), "\ncarton flags are:\n")
	f.PrintDefaults()
}

func (b *build) Run(ctx context.Context, args ...string) error {
	if len(args) == 0 {
		return commandLineErrorf("carton name must be supplied")
	}

	return load.NewLoad(b.name, b.Loaders).Run(ctx, args[0], b.Target, b.NoDeps)

}

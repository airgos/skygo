// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"context"
	"flag"
	"fmt"
	"os"

	"merge/carton"
	"merge/runbook"
)

type build struct {
	NoDeps bool   `flag:"nodeps" help:"don't check dependency"`
	Exec   string `flag:"play" help:"one indenpent task or stage name"`
	// TODO: -HEAD, -interactive
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

	c, _, e := carton.Find(args[0])
	if e != nil {
		return fmt.Errorf("carton %s is %s", args[0], e)
	}

	rb := c.Runbook()
	arg := new(runbook.Arg)
	arg.Stdout = os.Stdout
	arg.Stderr = os.Stderr
	arg.Owner = args[0]
	arg.Direnv = c.(runbook.DirEnv)

	ctx = runbook.NewContext(ctx, arg)
	if b.Exec != "" {
		return rb.Play(ctx, b.Exec)
	} else if b.NoDeps {
		return rb.Perform(ctx)
	}

	// w1, e := os.OpenFile("/dev/ttys002", os.O_RDWR, 0766)
	// w2, _ := os.OpenFile("/dev/ttys008", os.O_RDWR, 0766)
	// return carton.NewLoad(2, args[0]).SetOutput(0, w1, w1).SetOutput(1, w2, w2).Run(ctx)
	return carton.NewLoad(0, args[0]).Run(ctx)
}

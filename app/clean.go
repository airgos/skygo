package app

import (
	"context"
	"flag"
	"fmt"
	"merge/carton"
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

func (b *clean) Run(ctx context.Context, args ...string) error {
	if len(args) == 0 {
		return commandLineErrorf("carton name must be supplied")
	}

	c, _, e := carton.Find(args[0])
	if e != nil {
		return fmt.Errorf("carton %s is %s", args[0], e)
	}

	e = c.Clean(ctx, b.Force)
	if e == runbook.ErrUnknownTask {
		return fmt.Errorf("carton %s has no task clean, try to add flag -force", args[0])
	}
	return e
}

package app

import (
	"boxgo/carton"
	"flag"
	"fmt"
	"os"
)

type build struct {
	NoDeps bool   `flag:"nodeps" help:"don't check dependency"`
	Exec   string `flag:"play" help:"one indenpent task or stage name"`
	// TODO: -HEAD, -interactive
}

func (*build) Name() string    { return "carton" }
func (*build) Usage() string   { return "<carton name>" }
func (*build) Summary() string { return "build carton" }

func (*build) Help(f *flag.FlagSet) {

	fmt.Fprintf(f.Output(), "\ncarton flags are:\n")
	f.PrintDefaults()
}

func (b *build) Run(args ...string) error {
	if len(args) == 0 {
		return commandLineErrorf("carton name must be supplied")
	}

	carton.BuildInventory()
	c, _ := carton.Find(args[0])
	c.SetOutput(os.Stdout, os.Stderr)
	rb := c.Runbook()
	if b.Exec != "" {
		return rb.Play(b.Exec)
	} else if b.NoDeps {
		return rb.Perform()
	}

	return nil
}

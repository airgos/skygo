// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"context"
	"flag"
	"fmt"
	"merge/carton"
	"os"
)

// App implement interface Application
type App struct {
	name string
	// TODO: -log
}

// New create top Application
func New() Application {
	app := new(App)
	app.name = "merge"
	return app
}

// Name implement Application.Name
func (app *App) Name() string { return app.name }

// Summary implement Application.Summary
func (app *App) Summary() string { return "All-in-one frontend tool to help make embedded system easy" }

// UsageLine implement Application.UsageLine
func (app *App) UsageLine() string { return "<command> [command-flags] [command-args]" }

// Help implement Application.Help to print main help message
func (app *App) Help(f *flag.FlagSet) {
	fmt.Fprint(f.Output(), `
Available commands are:
`)
	for _, c := range app.commands() {
		fmt.Fprintf(f.Output(), "  %s : %s\n", c.Name(), c.Summary())
	}

	// fmt.Fprintf(f.Output(), "\n%s flags are:\n", app.Name())
	// f.PrintDefaults()
}

// Run takes the args after top level flag processing, and invokes the correct sub command
func (app *App) Run(ctx context.Context, args ...string) error {

	if len(args) == 0 {
		return commandLineErrorf("command must be supplied")
	}

	if err := carton.BuildInventory(ctx); err != nil {

		fmt.Println(err)
		os.Exit(1)
	}

	name, args := args[0], args[1:]
	for _, c := range app.commands() {
		if c.Name() == name {
			Main(ctx, c, args)
			return nil
		}
	}
	return commandLineErrorf("Unknown command %s", name)
}

func (*App) commands() []Application {
	return []Application{
		&info{},
		&build{},
		&clean{},
	}
}

// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"context"
	"flag"
	"fmt"
	"os"

	"merge/log"
)

// App implement interface Application
type App struct {
	name     string
	lockfile string
	LogLevel string `flag:"loglevel" help:"Log Level: trace, info, warning(default), error"`
}

// New create top app that implement Application
func New() *App {
	app := new(App)
	app.name = "merge"
	return app
}

// Clean cleanup App
func (app *App) Clean() { os.Remove(app.lockfile) }

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

	fmt.Fprintf(f.Output(), "\n%s flags are:\n", app.Name())
	f.PrintDefaults()
}

// Run takes the args after top level flag processing, and invokes the correct sub command
func (app *App) Run(ctx context.Context, args ...string) error {

	if app.LogLevel != "" {
		if !log.SetLevel(app.LogLevel) {
			return commandLineErrorf("Unknown log level %s", app.LogLevel)
		}
	}

	if len(args) == 0 {
		return commandLineErrorf("command must be supplied")
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

func (app *App) commands() []Application {
	return []Application{
		&info{name: app.name},
		&build{name: app.name},
		&clean{name: app.name},
	}
}

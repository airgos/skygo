package app

import (
	"boxgo/carton"
	"context"
	"flag"
	"fmt"
	"os"
)

// App implement interface Application
type App struct {
	name string
	// TODO: -log
}

// New create top Application
func New(name string) Application {
	app := new(App)
	app.name = name
	return app
}

// Name implement Application.Name
func (app *App) Name() string { return app.name }

// Summary implement Application.Summary
func (app *App) Summary() string { return "All-in-one frontend tool to help make embedded system easy" }

// Usage implement Application.Usage
func (app *App) Usage() string { return "<command> [command-flags] [command-args]" }

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

	if looped, loop := carton.BuildInventory(); looped {

		fmt.Printf("Loop found: %s", loop[0])
		for i := 1; i < len(loop); i++ {
			fmt.Printf(" --> %s", loop[i])
		}
		fmt.Println()
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
	}
}

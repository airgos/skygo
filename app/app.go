package app

import (
	"context"
	"flag"
	"fmt"
	"log"
	"reflect"
)

// Application is the interface
type Application interface {
	// Name returns the name of application
	Name() string

	// Usage returns synopsis  of application, but without flag argument
	Usage() string

	// Summary return short message to describe what the application can do
	Summary() string

	// Help pirnt detail help message
	// It is passed the flag set so it can print the default values of the flags.
	// It should use the flag sets configured Output to write the help to.
	Help(*flag.FlagSet)

	Run(ctx context.Context, args ...string) error
}

type commandLineError string

func (e commandLineError) Error() string {
	return string(e)
}

// commandLineErrorf is like fmt.Errorf except that it returns a value that
// triggers printing of the command line help.
// In general you should use this when generating command line validation errors.
func commandLineErrorf(message string, args ...interface{}) error {
	return commandLineError(fmt.Sprintf(message, args...))
}

// Main execute application
func Main(ctx context.Context, app Application, args []string) {

	s := flag.NewFlagSet(app.Name(), flag.ExitOnError)
	s.Usage = func() {

		fmt.Fprint(s.Output(), app.Summary())
		fmt.Fprintf(s.Output(), "\n\nUsage: %s [flags] %s\n", app.Name(), app.Usage())
		app.Help(s)
		// if flag have method to report how many flags are added, call s.PrintDefaults() here
	}

	value := reflect.ValueOf(app)
	addflags(s, value)

	s.Parse(args)
	e := app.Run(ctx, s.Args()...)
	if e != nil {
		fmt.Fprintf(s.Output(), "%s\n", e)
		if _, ok := e.(commandLineError); ok {
			s.Usage()
		}
	}
}

func addflags(s *flag.FlagSet, value reflect.Value) {

	t := value.Type().Elem()
	if t.Kind() != reflect.Struct {
		return
	}
	value = value.Elem()
	for i := 0; i < t.NumField(); i++ {

		flagName, isFlag := t.Field(i).Tag.Lookup("flag")
		if isFlag {

			help := t.Field(i).Tag.Get("help")

			v := value.Field(i)
			if v.Kind() != reflect.Ptr {
				v = v.Addr()
			}

			switch v := v.Interface().(type) {
			case *string:
				s.StringVar(v, flagName, *v, help)
			case *bool:
				s.BoolVar(v, flagName, *v, help)
			default:
				log.Fatalf("Cannot understand flag of type %T", v)
			}
		}
	}
}

package carton

import (
	"io"

	"merge/fetch"
	"merge/runbook"
)

// Modifier is the interface to help modify carton
type Modifier interface {
	AddFilePath(dir string) error

	SetSrcPath(dir string) error

	// used to update dependency
	Depends(dep ...string) []string
	BuildDepends(dep ...string) []string

	// Runbook give runbook
	Runbook() *runbook.Runbook

	// Return fetch state
	Resource() *fetch.Resource
}

// BuildYard is the interface to provide environment where Builder work
type BuildYard interface {
	// SrcPath return  directory of source code
	SrcPath() string

	// FilePath return a collection of directory that's be used for locating url scheme file:
	FilePath() []string

	// Environ returns a slice of strings representing the environment,
	// in the form "key=value".
	Environ() []string

	Output() (stdout, stderr io.Writer)
}

// Builder is the interface to build a carton
type Builder interface {

	// Provider returns what's provided. Provider can be software, image etc
	Provider() string

	// From returns which files describe this carton if no argument
	// if file parameter is given, only the first index will be used to record
	// who escribes the carton
	From(file ...string) []string

	// BuildDepends add depends only required for building from scratch
	// Always return the same kind of depends
	BuildDepends(dep ...string) []string

	// Depends add depends required for building from scratch, running or both
	// Always return the same kind of depends
	Depends(dep ...string) []string

	// Runbook return runbook
	Runbook() *runbook.Runbook

	// Return fetch state
	Resource() *fetch.Resource

	// SetOutput set stdout, stderr. Runbook's task need it
	SetOutput(stdout, stderr io.Writer)

	WorkPath() string
	BuildYard

	String() string
}

// Package carton implements ...

// Dependency Format: carton-name[@procedure]

package carton

import (
	"io"

	"boxgo/runbook"
)

// Initer xx
type Initer interface {
	Init()
	InstallRunbook()
}

// Modifier is the interface to help modify carton
type Modifier interface {
	AddFilePath(dir string) error

	AddHeadSrc(srcURL string)
	AddSrcURL(versiong string, srcURL string)
	PreferSrcURL(version string)
	SetSrcPath(dir string) error

	// used to update dependency
	Depends(dep ...string) []string
	BuildDepends(dep ...string) []string

	// used to modify task for stage & independent taskset
	TaskSet() *runbook.TaskSet
	Stage(name string) *runbook.Stage
}

// BuildYard is the interface to provide environment where Builder work
// TODO: merge with fetch and taskcmd
// merge SrcPath to WorkPath ?
type BuildYard interface {
	SrcPath() string
	FilePath() []string
	Environ() []string
	Output() (stdout, stderr io.Writer)
}

// Builder is the interface to build a carton
type Builder interface {
	Provider() string
	Versions() []string
	String() string
	From(file ...string) []string

	WorkPath() string

	Depends(dep ...string) []string
	BuildDepends(dep ...string) []string

	Play(name string) error
	Perform() error

	// RunbookInfo give stage sequence with the number of each stage
	RunbookInfo() ([]string, []int, []string)

	SetOutput(stdout, stderr io.Writer)

	BuildYard

	CloneRunbook(r runbook.Runtime) *runbook.Runbook
}

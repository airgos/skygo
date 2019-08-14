// Package carton implements ...

// Dependency Format: carton-name[@procedure]

package carton

import (
	"io"

	"boxgo/runbook"
)

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

	// give runbook
	Runbook() *runbook.Runbook
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

	// give runbook
	Runbook() *runbook.Runbook

	SetOutput(stdout, stderr io.Writer)

	BuildYard
}

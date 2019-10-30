// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package carton

import (
	"merge/fetch"
	"merge/runbook"
)

// Modifier is the interface to help modify carton
type Modifier interface {

	// AddFilePath appends one path to FilesPath if it does exist
	// dir will be joined with the directory path of caller(which
	// file invokes AddFilePath)
	AddFilePath(dir string) error

	// SetSrcDir set source directory explicitily.
	// dir can be relative or absolute path
	// relative path must be under WORKDIR or under caller's directory path
	// it's useful to support absolute path for development
	// if dir has prefix '~', it will be extended
	SetSrcDir(dir string) error

	// used to update dependency
	// dep format: cartonName[@stageName] or carton group
	Depends(dep ...string) []string
	BuildDepends(dep ...string) []string

	runbook.KVSetter

	// Runbook give runbook
	Runbook() *runbook.Runbook

	// Return fetch state
	Resource() *fetch.Resource
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
	// dep format: cartonName[@stageName] or carton group
	// carton group is a collection of cartonName[@stageName] with delimiter space
	// Always return the same kind of depends
	BuildDepends(...string) []string

	// Depends add depends required for building from scratch, running or both
	// dep format: cartonName[@stageName] or carton group
	// carton group is a collection of cartonName[@stageName] with delimiter space
	// Always return the same kind of depends
	Depends(...string) []string

	// Runbook return runbook
	Runbook() *runbook.Runbook

	// Return fetch state
	Resource() *fetch.Resource

	runbook.KVGetter

	// return where source code is under WORKDIR
	SrcDir(wd string) string

	// FilesPath return a collection of directory that's be used for locating local file
	FilesPath() []string

	String() string
}

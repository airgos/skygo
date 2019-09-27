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

	// appends one path to File Path
	// dir will be joined with directory path of which file invokes AddFilePath
	AddFilePath(dir string) error

	// Set where source code is explicitly
	SetSrcPath(dir string) error

	// used to update dependency
	// dep format: cartonName[@stageName]
	Depends(dep ...string) []string
	BuildDepends(dep ...string) []string

	// GetVar retrieves the value of the variable named by the key.
	// It returns the value, which will be empty if the variable is not present.
	GetVar(key string) string

	// SetVar sets the value of the variable named by the key.
	SetVar(key, value string)

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
	// Always return the same kind of depends
	BuildDepends(dep ...string) []string

	// Depends add depends required for building from scratch, running or both
	// Always return the same kind of depends
	Depends(dep ...string) []string

	// Runbook return runbook
	Runbook() *runbook.Runbook

	// Return fetch state
	Resource() *fetch.Resource

	// GetVar retrieves the value of the variable named by the key.
	// It returns the value, which will be empty if the variable is not present.
	GetVar(key string) string

	// VisitVars visit each variable
	VisitVars(f func(key, value string))

	// return where source code is under WORKDIR
	SrcPath(wd string) string

	// FilesPath return a collection of directory that's be used for locating local file
	FilesPath() []string

	String() string
}

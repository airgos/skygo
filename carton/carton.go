// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package carton implements interface Builder and Modifier
package carton

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"merge/config"
	"merge/fetch"
	"merge/log"
	"merge/runbook"
)

// Error used by carton
var (
	ErrNotFound = errors.New("Not Found")
	ErrNoName   = errors.New("Illegal Provider")
	ErrAbsPath  = errors.New("Abs Path")
)

// predefined stage
const (
	FETCH   = "fetch"
	PATCH   = "patch"
	PREPARE = "prepare"
	BUILD   = "build"
	INSTALL = "install"
	TEST    = "test"
)

// The Carton represents the state of carton
// It implements interface Builder and Modifier
type Carton struct {
	Desc     string // oneline description
	Homepage string // home page

	name    string
	cartons []string

	file      []string // which files offer this carton
	srcpath   string   // path(dir) of SRC code
	filespath []string // search dirs for scheme file://

	depends      []string // needed for both running and building
	buildDepends []string // only needed when building from scratch

	fetch   *fetch.Resource
	runbook *runbook.Runbook

	vars map[string]string
}

// NewCarton create a carton and add to inventory
func NewCarton(name string, m func(c *Carton)) {

	c := new(Carton)
	c.name = name
	_, file, _, _ := runtime.Caller(1)

	c.Init(file, c, func(arg Modifier) {

		rb := runbook.NewRunbook()
		fetch := rb.PushFront(FETCH)
		fetch.AddTask(0, func(ctx context.Context) error {
			arg, _ := runbook.FromContext(ctx)
			os.MkdirAll(arg.Wd, 0755)
			return c.fetch.Download(ctx,
				// reset subsequent stages
				func(ctx context.Context) {
					log.Trace("Reset subsequent stages because fetch found new code")
					for stage := fetch.Next(); stage != nil; stage = stage.Next() {
						stage.Reset(ctx)
					}
				})
		})

		fetch.InsertAfter(PATCH).
			InsertAfter(PREPARE).InsertAfter(BUILD).InsertAfter(INSTALL)
		c.runbook = rb

		m(c)
	})
}

// Init initialize carton and add to inventory
// install runbook in callback modify
func (c *Carton) Init(file string, arg Modifier, modify func(arg Modifier)) {

	add(c, file, func() {
		c.cartons = []string{}
		c.vars = make(map[string]string)
		c.fetch = fetch.NewFetch(config.GetVar("DLDIR"))

		c.file = []string{}
		c.filespath = []string{}

		c.cartons = append(c.cartons, c.name)
		c.vars["CN"] = c.name //CN: carton name

		modify(arg)
	})
}

// Provider return what's provided
func (c *Carton) Provider() string {
	return c.name
}

// From add new location indicating which file provide carton
// Return location list
func (c *Carton) From(file ...string) []string {

	notAdded := func(from string) bool {
		for _, f := range c.file {
			if f == from {
				return false
			}
		}
		return true
	}

	if len(file) != 0 {

		if from := file[0]; from != "" {

			if notAdded(from) {
				c.file = append(c.file, from)
				filepath := strings.TrimSuffix(from, ".go")
				c.filespath = append(c.filespath, filepath)
			}
		}
	}

	return c.file
}

// BuildDepends add depends only required for building from scratch
// dep format: cartonName[@stageName]
// Always return the same kind of depends
func (c *Carton) BuildDepends(dep ...string) []string {

	if len(dep) == 0 {
		return c.buildDepends
	}
	c.buildDepends = append(c.buildDepends, dep...)
	return c.buildDepends
}

// Depends add depends required for building from scratch, running or both
// dep format: cartonName[@stageName]
// Always return the same kind of depends
func (c *Carton) Depends(dep ...string) []string {

	if len(dep) == 0 {
		return c.depends
	}
	c.depends = append(c.depends, dep...)
	return c.depends
}

// SrcDir return where source code is under WORKDIR
// WORKDIR depends on ARCH. one carton has different WORKDIR for different ARCH
func (c *Carton) SrcDir(wd string) string {

	if c.srcpath != "" {
		d := filepath.Join(wd, c.srcpath)
		if info, e := os.Stat(d); e != nil || !info.IsDir() {
			log.Error("SRCDIR %s does not existed", d)
		}
		return d
	}

	d := filepath.Join(wd, c.name)
	if info, e := os.Stat(d); e == nil && info.IsDir() {
		return d
	}

	_, ver := c.Resource().Selected()
	d = filepath.Join(wd, fmt.Sprintf("%s-%s", c.name, ver))
	if info, e := os.Stat(d); e == nil && info.IsDir() {
		return d
	}

	log.Warning("Don't know where SrcPath is. Please set it by SetSrcPath explicitily")
	return ""
}

// SetSrcDir set SrcDir explicitily. dir msut be a relative path that's under WORKDIR
func (c *Carton) SetSrcDir(dir string) error {
	if filepath.IsAbs(dir) {
		return ErrAbsPath
	}
	c.srcpath = dir
	return nil
}

// AddFilePath appends one path to File Path
// dir will be joined with directory path of which file invokes AddFilePath
func (c *Carton) AddFilePath(dir string) error {

	if filepath.IsAbs(dir) {
		return ErrAbsPath
	}
	_, file, _, _ := runtime.Caller(1)
	dir = filepath.Join(filepath.Dir(file), dir)
	_, e := os.Stat(dir)
	if e == nil {

		c.filespath = append(c.filespath, dir)
	}
	return e
}

// FilesPath return FilePath
func (c *Carton) FilesPath() []string {
	return c.filespath
}

// Resource return fetch state
func (c *Carton) Resource() *fetch.Resource {
	return c.fetch
}

// Runbook return runbook hold by Carton
func (c *Carton) Runbook() *runbook.Runbook {
	return c.runbook
}

// SetRunbook assign runbook
func (c *Carton) SetRunbook(rb *runbook.Runbook) {
	c.runbook = rb
}

// LookupVar retrieves the value of the variable named by the key.
// If the variable is present, value (which may be empty) is returned
// and the boolean is true. Otherwise the returned value will be empty
// and the boolean will be false.
func (c *Carton) LookupVar(key string) (string, bool) {
	value, ok := c.vars[key]
	return value, ok
}

// SetVar sets the value of the variable named by the key.
func (c *Carton) SetVar(key, value string) {
	c.vars[key] = value
}

// VisitVars visit each variable
func (c *Carton) VisitVars(f func(key, value string)) {
	for k, v := range c.vars {
		f(k, v)
	}
}

func (c *Carton) String() string {

	var b strings.Builder

	if c.Desc != "" {
		fmt.Fprintf(&b, "%s\n", c.Desc)
	}

	if c.Homepage != "" {
		fmt.Fprintf(&b, "%s\n", c.Homepage)
	}

	if len(c.cartons) > 0 {
		fmt.Fprintf(&b, "Provids: %s", c.cartons[0])
		for _, p := range c.cartons[1:] {
			fmt.Fprintf(&b, " %s", p)
		}
		fmt.Fprintf(&b, "\n")
	}

	// where come from
	if len(c.file) > 0 {
		fmt.Fprintf(&b, "From: %s\n", c.file[0])
		for _, file := range c.file[1:] {
			fmt.Fprintf(&b, "      %s\n", file)
		}
	}

	return b.String()
}

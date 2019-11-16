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

	"skygo/fetch"
	"skygo/pkg"
	"skygo/runbook"
	"skygo/utils/log"
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
	PACKAGE = "package"
)

// The Carton represents the state of carton
// It implements interface Builder and Modifier
type Carton struct {
	Desc     string // oneline description
	Homepage string // home page

	name    string
	cartons []string

	file      []string // which files offer this carton
	srcdir    string   // path(dir) of Source code, value of var S
	filespath []string // search dirs for scheme file://

	depends      []string // needed for both running and building
	buildDepends []string // only needed when building from scratch

	fetch   *fetch.Resource
	runbook *runbook.Runbook

	runbook.KV // embed key-value

	pkg.Packages // packager
}

// NewCarton create a carton and add to inventory
func NewCarton(name string, m func(c *Carton)) {

	c := new(Carton)
	c.name = name
	_, file, _, _ := runtime.Caller(1)

	c.Init(file, c, func(arg Modifier) {

		rb := runbook.NewRunbook()
		fetch := rb.PushFront(FETCH).Summary("Fetchs the source code and extract")
		fetch.AddTask(0, func(ctx context.Context, dir string) error {
			return c.fetch.Download(ctx,
				// reset subsequent stages
				func(ctx context.Context) {
					log.Trace("Reset subsequent stages because fetch found new code")
					for stage := fetch.Next(); stage != nil; stage = stage.Next() {
						stage.Reset(ctx)
					}
				})
		})

		fetch.InsertAfter(PATCH).Summary("Locates patch files and applies them to the source code").
			InsertAfter(PREPARE).Summary("Prepares something for build").
			InsertAfter(BUILD).Summary("Compiles the source in the compilation directory").
			InsertAfter(INSTALL).Summary("Install files from the compilation directory").
			InsertAfter(PACKAGE).Summary("Package files from the installation directory").
			AddTask(0, func(ctx context.Context, dir string) error {
				arg := runbook.FromContext(ctx)
				return c.Package(arg.GetVar("D"), arg.GetVar("PKGD"))
			})

		c.runbook = rb

		m(c)
	})
}

// Init initialize carton and add to inventory
// install runbook in callback modify
func (c *Carton) Init(file string, arg Modifier, modify func(arg Modifier)) {

	add(c, file, func() {
		c.cartons = []string{}
		c.fetch = fetch.NewFetch()

		c.file = []string{}
		c.filespath = []string{}

		c.cartons = append(c.cartons, c.name)
		c.KV.Init(c.name)
		c.SetKv("CN", c.name) //CN: carton name. By default, CN is the same as PN(c.name, provider name)

		// create two packages: provider, provider-dev
		c.NewPkg(c.name)
		c.NewPkg(c.name + "-dev")

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
// dep format: cartonName[@stageName] or carton group
// carton group is a collection of cartonName[@stageName] with delimiter space
// Always return the same kind of depends
func (c *Carton) BuildDepends(deps ...string) []string {

	if len(deps) == 0 {
		return c.buildDepends
	}
	for _, dep := range deps {
		for _, d := range strings.Fields(dep) {
			c.buildDepends = append(c.buildDepends, d)
		}
	}
	return c.buildDepends
}

// Depends add depends required for building from scratch, running or both
// dep format: cartonName[@stageName] or carton group
// carton group is a collection of cartonName[@stageName] with delimiter space
// Always return the same kind of depends
func (c *Carton) Depends(deps ...string) []string {

	if len(deps) == 0 {
		return c.depends
	}
	for _, dep := range deps {
		for _, d := range strings.Fields(dep) {
			c.depends = append(c.depends, d)
		}
	}
	return c.depends
}

// SrcDir return where source code is under WORKDIR
// WORKDIR depends on ARCH. one carton has different WORKDIR for different ARCH
func (c *Carton) SrcDir(wd string) string {

	if filepath.IsAbs(c.srcdir) {
		return c.srcdir
	}

	if c.srcdir != "" { // SrcDir is configured explicitily by SetSrcDir
		// don't check its existence
		return filepath.Join(wd, c.srcdir)
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

	return ""
}

// SetSrcDir set source directory explicitily.
// dir can be relative or absolute path
// relative path must be under WORKDIR or under caller's directory path
// it's useful to support absolute path for development
// if dir has prefix '~', it will be extended
func (c *Carton) SetSrcDir(dir string) error {

	if strings.HasPrefix(dir, "~") {
		dir = strings.Replace(dir, "~", os.Getenv("HOME"), 1)
	}

	// try to find dir under Caller's path
	// if found, absolute path will be assigned to srcdir
	if !filepath.IsAbs(dir) {
		_, file, _, _ := runtime.Caller(1)
		r := filepath.Join(filepath.Dir(file), dir)
		if _, e := os.Stat(r); e == nil {
			dir = r
		}
	}
	c.srcdir = dir
	return nil
}

// AddFilePath appends one path to FilesPath if it does exist
// dir will be joined with the directory path of caller(which
// file invokes AddFilePath)
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
		fmt.Fprintf(&b, "   From: %s\n", c.file[0])
		for _, file := range c.file[1:] {
			fmt.Fprintf(&b, "         %s\n", file)
		}
	}

	return b.String()
}

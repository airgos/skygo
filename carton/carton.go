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

	name     string
	provider []string

	file     []string // which files offer this carton
	srcpath  string   // path(dir) of SRC code
	filepath []string // search dirs for scheme file://

	depends      []string // needed for both running and building
	buildDepends []string // only needed when building from scratch

	fetch   *fetch.Resource
	runbook *runbook.Runbook

	// environment variables who are exported to cartion running space by format key=value
	environ map[string]string
}

// NewCarton create a carton and add to inventory
func NewCarton(name string, m func(c *Carton)) {

	c := new(Carton)
	c.name = name
	_, file, _, _ := runtime.Caller(1)

	c.Init(file, c, func(arg Modifier) {

		chain := runbook.NewRunbook()
		p, _ := chain.PushFront(FETCH).AddTask(0, func(ctx context.Context) error {
			return c.fetch.Download(ctx)
		})
		p, _ = p.InsertAfter(PATCH).AddTask(0, func(ctx context.Context) error {
			return Patch(ctx, c)
		})
		p.InsertAfter(PREPARE).InsertAfter(BUILD).InsertAfter(INSTALL)
		c.runbook = chain

		m(c)
	})
}

// Init initialize carton and add to inventory
// install runbook in callback modify
func (c *Carton) Init(file string, arg Modifier, modify func(arg Modifier)) {

	add(c, file, func() {
		c.provider = []string{}
		c.environ = make(map[string]string)
		b := arg.(Builder)
		c.fetch = fetch.NewFetch(config.DownloadDir(), b.WorkPath(), b.FilePath())

		c.file = []string{}
		c.filepath = []string{}

		c.provider = append(c.provider, c.name)
		c.environ["PN"] = c.name

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
				c.filepath = append(c.filepath, filepath)
			}
		}
	}

	return c.file
}

// BuildDepends add depends only required for building from scratch
// Always return the same kind of depends
func (c *Carton) BuildDepends(dep ...string) []string {

	if len(dep) == 0 {
		return c.buildDepends
	}
	c.buildDepends = append(c.buildDepends, dep...)
	return c.buildDepends
}

// Depends add depends required for building from scratch, running or both
// Always return the same kind of depends
func (c *Carton) Depends(dep ...string) []string {

	if len(dep) == 0 {
		return c.depends
	}
	c.depends = append(c.depends, dep...)
	return c.depends
}

// SrcPath give under which source code is
func (c *Carton) SrcPath() string {

	if c.srcpath != "" {

		return c.srcpath
	}

	if file, e := os.Open(c.WorkPath()); e == nil {
		var d string
		if fpaths, e := file.Readdirnames(-1); e == nil {

			// choose the only one dir
			if len(fpaths) == 1 {
				d = filepath.Join(c.WorkPath(), fpaths[0])
				if info, e := os.Stat(d); e == nil && info.IsDir() {
					c.srcpath = d
					return d
				}
			}

			_, ver := c.Resource().Selected()
			d = filepath.Join(c.WorkPath(), fmt.Sprintf("%s-%s", c.Provider(), ver))
			if info, e := os.Stat(d); e == nil && info.IsDir() {
				c.srcpath = d
				return d
			}

			if len(fpaths) > 1 {
				log.Warning("Don't know which directory should be chosen for SrcPath. Please set it explicitily")
			}
		}
	}
	return ""
}

// SetSrcPath set SrcPath explicitily. It joins with output of WorkPath() as SrcPath
func (c *Carton) SetSrcPath(dir string) error {
	if filepath.IsAbs(dir) {
		return ErrAbsPath
	}
	c.srcpath = filepath.Join(c.WorkPath(), dir)
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

		c.filepath = append(c.filepath, dir)
	}
	return e
}

// FilePath return FilePath
func (c *Carton) FilePath() []string {
	return c.filepath
}

// Resource return fetch state
func (c *Carton) Resource() *fetch.Resource {
	return c.fetch
}

// WorkPath return value of WorkPath
func (c *Carton) WorkPath() string {

	dir := fmt.Sprintf("%s", c.Provider())
	// TODO: get from config package
	dir = filepath.Join("build", dir)
	dir, _ = filepath.Abs(dir)
	if _, e := os.Stat(dir); e != nil {
		os.MkdirAll(dir, 0755)
	}
	return dir
}

// Runbook return runbook hold by Carton
func (c *Carton) Runbook() *runbook.Runbook {
	return c.runbook
}

// SetRunbook assign runbook
func (c *Carton) SetRunbook(rb *runbook.Runbook) {
	c.runbook = rb
}

// Environ returns a copy of strings representing the environment,
// in the form "key=value".
func (c *Carton) Environ() []string {
	env := make([]string, 0, len(c.environ))
	for k, v := range c.environ {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	_, ver := c.Resource().Selected()
	env = append(env, fmt.Sprintf("PV=%s", ver), fmt.Sprintf("SRC=%s", c.SrcPath()))
	return env
}

// Setenv sets the value of the environment variable named by the key.
// It returns an error, if any.
func (c *Carton) Setenv(key, value string) {
	c.environ[key] = value
}

// Clean cleanup
// if force is true, remove work path; else try to run independent task clean
func (c *Carton) Clean(ctx context.Context, force bool) error {
	if force {
		wd := c.WorkPath()
		os.RemoveAll(wd)
		return nil
	}
	tset := c.Runbook().TaskSet()
	if tset.Has("clean") {
		return tset.Run(ctx, "clean")
	}
	return runbook.ErrUnknownTask
}

func (c *Carton) String() string {

	var b strings.Builder

	if c.Desc != "" {
		fmt.Fprintf(&b, "%s\n", c.Desc)
	}

	if c.Homepage != "" {
		fmt.Fprintf(&b, "%s\n", c.Homepage)
	}

	if len(c.provider) > 0 {
		fmt.Fprintf(&b, "Provider: %s", c.provider[0])
		for _, p := range c.provider[1:] {
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

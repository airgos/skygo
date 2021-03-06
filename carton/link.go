// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package carton

import (
	"fmt"
	"runtime"
	"skygo/fetch"
	"skygo/runbook"
	"strings"
)

// link represent a link of carton
// designed for virtual carton and multiple provider carton
// it only implements Builder interface
type link struct {
	alias string
	h     Builder
}

func (l *link) Provider() string                    { return l.h.Provider() }
func (l *link) Resource() *fetch.Resource           { return l.h.Resource() }
func (l *link) From(file ...string) []string        { return l.h.From(file...) }
func (l *link) SrcDir(wd string) string             { return l.h.SrcDir(wd) }
func (l *link) FilesPath() []string                 { return l.h.FilesPath() }
func (l *link) BuildDepends(dep ...string) []string { return l.h.BuildDepends() }
func (l *link) Depends(dep ...string) []string      { return l.h.Depends() }
func (l *link) Runbook() *runbook.Runbook           { return l.h.Runbook() }
func (l *link) String() string                      { return l.h.String() }

// Get retrieves the value of the variable named by the key.
// If the variable is not present, nil is returned
func (l *link) Get(key string) interface{} {

	if key == "CN" {
		return l.alias
	}
	return l.h.Get(key)
}

// Range visits each item in KV
func (l *link) Range(f func(key, value string)) {
	l.h.Range(func(key, value string) {
		if key == "CN" {
			value = l.alias // overwrite value of key CN
		}
		f(key, value)
	})
}

// Provide create link to provider
func (c *Carton) Provide(provider ...string) {

	pc, file, _, _ := runtime.Caller(1)
	details := runtime.FuncForPC(pc)
	if !strings.Contains(details.Name(), ".init.") {

		panic(fmt.Errorf("%s: must add provider in init func", file))
	}

	for _, name := range provider {

		link := link{h: c, alias: name}
		addVirtual(&link, name, file)
		c.cartons = append(c.cartons, name)
	}
}

// Link forcely link a real Carton
func Link(h Builder, target string) {

	// not allow create a link based on a link
	if _, ok := h.(Modifier); !ok {
		return
	}

	_, file, _, _ := runtime.Caller(1)
	link := link{h: h, alias: target}
	addVirtual(&link, target, file)
}

// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package carton

import (
	"context"
	"fmt"
	"merge/fetch"
	"merge/runbook"
	"runtime"
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
func (l *link) SrcPath(wd string) string            { return l.h.SrcPath(wd) }
func (l *link) FilesPath() []string                 { return l.h.FilesPath() }
func (l *link) BuildDepends(dep ...string) []string { return l.h.BuildDepends() }
func (l *link) Depends(dep ...string) []string      { return l.h.Depends() }
func (l *link) Runbook() *runbook.Runbook           { return l.h.Runbook() }
func (l *link) String() string                      { return l.h.String() }

func (l *link) SetVar(key, value string) { l.SetVar(key, value) }

// GetVar retrieves the value of the variable named by the key.
// It returns the value, which will be empty if the variable is not present.
func (l *link) GetVar(key string) string {
	if key == "CN" {
		return l.alias
	}
	return l.GetVar(key)
}

// VisitVars visit each variable
func (l *link) VisitVars(f func(key, value string)) {
	l.VisitVars(func(key, value string) {
		if key == "CN" {
			value = l.alias
		}
		f(key, value)
	})
}

func (l *link) Clean(ctx context.Context, force bool) error {
	return l.h.Clean(ctx, force)
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

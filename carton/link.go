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
	alias   string
	runbook *runbook.Runbook
	h       Builder
}

func (l *link) Provider() string                    { return l.h.Provider() }
func (l *link) Resource() *fetch.Resource           { return l.h.Resource() }
func (l *link) From(file ...string) []string        { return l.h.From(file...) }
func (l *link) SrcPath() string                     { return l.h.SrcPath() }
func (l *link) WorkPath() string                    { return l.h.WorkPath() }
func (l *link) FilePath() []string                  { return l.h.FilePath() }
func (l *link) BuildDepends(dep ...string) []string { return l.h.BuildDepends() }
func (l *link) Depends(dep ...string) []string      { return l.h.Depends() }
func (l *link) Runbook() *runbook.Runbook           { return l.runbook }
func (l *link) String() string                      { return l.h.String() }

func (l *link) Clean(ctx context.Context, force bool) error {
	return l.h.Clean(ctx, force)
}

func (l *link) Environ() []string {
	return append(l.h.Environ(), fmt.Sprintf("PN=%s", l.alias))
}

// Provide create link to provider
func (c *Carton) Provide(provider ...string) {

	pc, file, _, _ := runtime.Caller(1)
	details := runtime.FuncForPC(pc)
	if !strings.Contains(details.Name(), ".init.") {

		panic(fmt.Errorf("%s: must add provider in init func", file))
	}

	rb := c.Runbook()
	for _, name := range provider {

		link := link{h: c, alias: name}
		link.runbook = rb.Clone(&link)
		addVirtual(&link, name, file)
		c.provider = append(c.provider, name)
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

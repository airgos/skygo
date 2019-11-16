// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg

import (
	"os"
	"path/filepath"

	"skygo/utils"
	"skygo/utils/log"
)

var pn = [...]string{
	"usr/bin",
	"usr/sbin",
	"bin",
	"sbin",
	"usr/lib",
	"lib",
	"etc",
}

var pn_dev = [...]string{
	"usr/include",
	"usr/lib",
	"lib",
}

type pkg struct {
	box  *utils.StageBox
	name string // individual package name
}

// Packges represents the state of package
type Packages struct {
	owner string
	pkgs  map[string]pkg
}

// NewPkg create new stageBox for package @name and add into Packages
func (p *Packages) NewPkg(name string) *utils.StageBox {
	if p.pkgs == nil {
		p.owner = name
		p.pkgs = make(map[string]pkg)
	}

	box := new(utils.StageBox)
	pkg := pkg{box: box, name: name}

	devpkg := p.owner + "-dev"

	switch name {
	case p.owner:
		for _, v := range pn {
			box.Push(v)
		}
	case devpkg:
		for _, v := range pn_dev {
			box.Push(v)
		}
		pkg.name = devpkg
	}

	p.pkgs[pkg.name] = pkg
	return box
}

// GetPkg get package @name from Packages
func (p *Packages) GetPkg(name string) *utils.StageBox {
	return p.pkgs[name].box
}

// Package pack files from @from to @to
func (p *Packages) Package(from, to string) error {

	for _, pkg := range p.pkgs {
		dest := filepath.Join(to, pkg.name)
		log.Info("Start staging files from %s to %s\n", from, dest)
		os.RemoveAll(dest)
		if err := pkg.box.Stage(from, dest); err != nil {
			return err
		}

		// TODO: pack
	}
	return nil
}

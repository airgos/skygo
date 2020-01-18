// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package load

import (
	"context"
	"path/filepath"

	"skygo/carton"
	"skygo/runbook"
	"skygo/runbook/xsync"
	"skygo/utils"
)

type cartonRequired struct {
	isNative bool
	c        carton.Builder
}

func depTree(c carton.Builder, isNative bool) map[string]cartonRequired {

	tree := map[string]cartonRequired{}
	walk(c, isNative, tree)
	return tree
}

func walk(c carton.Builder, isNative bool, tree map[string]cartonRequired) {

	d := c.BuildDepends()
	for _, v := range d {
		if c, _, native, err := carton.Find(v); err == nil {
			if native {
				isNative = true
			}
			tree[v] = cartonRequired{isNative: isNative, c: c}
			walk(c, isNative, tree)
		}
	}
}

// it does not care value of dir
func prepare_sysroot(ctx context.Context, dir string) error {

	arg := runbook.FromContext(ctx)
	carton := arg.Private.(carton.Builder)
	isNative := arg.Get("ISNATIVE").(bool)

	dest := filepath.Join(arg.GetStr("WORKDIR"), "sysroot")

	g, ctx := xsync.WithContext(ctx)
	for _, d := range depTree(carton, isNative) {

		d := d
		g.Go(func() error {
			wd := WorkDir(d.c, d.isNative)
			sysroot := dest
			n := d.c.Provider()
			if d.isNative {
				sysroot = dest + "-native"
			} else {
				n = n + "-dev"
			}
			from := filepath.Join(wd, "packages", n)
			return utils.Stage(from, sysroot)
		})
	}
	return g.Wait()
}

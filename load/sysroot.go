// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package load

import (
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
func prepare_sysroot(ctx runbook.Context) error {

	carton := getCartonFromCtx(ctx)
	isNative := ctx.Get("ISNATIVE").(bool)

	dest := filepath.Join(ctx.GetStr("WORKDIR"), "sysroot")

	g, _ := xsync.WithContext(ctx.Ctx())
	for _, d := range depTree(carton, isNative) {

		d := d
		g.Go(func() error {
			wd := workDir(d.c, d.isNative)
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

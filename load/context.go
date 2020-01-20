// Copyright © 2020 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// implements runbook.Context

package load

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"

	"skygo/carton"
	"skygo/runbook"
)

type _context struct {
	load   *Load
	carton carton.Builder
	kv     runbook.KV
	pool   *pool
}

func newContext(load *Load, carton carton.Builder,
	isNative bool) *_context {

	ctx := &_context{
		load:   load,
		carton: carton,
	}

	workDir := WorkDir(carton, isNative)
	tempDir := filepath.Join(workDir, "temp")
	destDir := filepath.Join(workDir, "image")   // install destination directory
	pkgDir := filepath.Join(workDir, "packages") // points to directory for files to be packaged

	os.MkdirAll(workDir, 0755)
	os.MkdirAll(tempDir, 0755)
	os.MkdirAll(destDir, 0755)
	os.MkdirAll(pkgDir, 0755)

	// key-value for each carton's context
	ctx.kv.Init2("context", map[string]interface{}{
		"WORKDIR":  workDir,
		"ISNATIVE": isNative,

		"PN":   carton.Provider(), // PN: provider name
		"T":    tempDir,
		"D":    destDir,
		"PKGD": pkgDir,

		"TARGETARCH":   getTargetArch(carton, isNative),
		"TARGETOS":     getTargetOS(carton, isNative),
		"TARGETVENDOR": getTargetVendor(carton, isNative),
	})

	if dir := carton.SrcDir(workDir); dir != "" {
		ctx.kv.Set("S", dir)
	}
	return ctx
}

func (ctx *_context) Ctx() context.Context {
	return ctx.load.ctx
}

func (ctx *_context) Owner() string {
	return ctx.carton.Provider()
}

func (ctx *_context) FilesPath() []string {
	return ctx.carton.FilesPath()
}

func (ctx *_context) Wait(runbook, stage string, isNative bool) <-chan struct{} {
	return ctx.load.Wait(runbook, stage, isNative)
}

func (ctx *_context) Private() interface{} {
	return ctx.carton
}

func (ctx *_context) Output() (stdout, stderr io.Writer) {

	if v := ctx.Get("STDERR"); v != nil {
		stderr = v.(io.Writer)
	}
	if ctx.pool.stderr != nil {
		stderr = io.MultiWriter(ctx.pool.stderr, stderr)
	}

	if v := ctx.Get("STDOUT"); v != nil {
		stdout = v.(io.Writer)
	}
	if ctx.pool.stdout != nil {
		stdout = io.MultiWriter(ctx.pool.stdout, stdout)
	}
	return
}

func (ctx *_context) Set(key string, value interface{}) {
	ctx.kv.Set(key, value)
}

func (ctx *_context) GetStr(key string) string {
	if v := ctx.Get(key); v != nil {
		if v, ok := v.(string); ok {
			return v
		}
	}

	return ""
}

func (ctx *_context) Get(key string) interface{} {
	if v := ctx.kv.Get(key); v != nil {
		return v
	}

	if v := ctx.carton.Get(key); v != nil {
		return v
	}

	if v := ctx.load.kv.Get(key); v != nil {
		return v
	}
	return nil
}

func (ctx *_context) Range(f func(key, value string)) {

	ctx.load.kv.Range(f)
	ctx.carton.Range(f)
	ctx.kv.Range(f)
}

func (ctx *_context) Acquire() error {

	y, err := ctx.load.pool.Get(ctx.Ctx())
	if err != nil {
		return err
	}

	x := y.(*pool)
	x.buf.Reset() // reset buffer
	ctx.pool = x

	return nil
}

func (ctx *_context) Release() {
	ctx.load.pool.Put(ctx.pool)
}

func (ctx *_context) errBuf() *bytes.Buffer {
	if ctx.pool == nil {
		return nil
	}
	return ctx.pool.buf
}

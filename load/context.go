// Copyright © 2020 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// implements runbook.Context

package load

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"skygo/carton"
	"skygo/runbook"
	"skygo/utils"
)

type _context struct {
	load   *Load
	carton carton.Builder
	kv     runbook.KV
	pool   *pool
	stage  string // running at which stage
}

func getCartonFromCtx(ctx runbook.Context) carton.Builder {

	return ctx.(*_context).carton
}

func getLoadFromCtx(ctx runbook.Context) *Load {

	return ctx.(*_context).load
}

// set which stage is currently running
func setStageToCtx(ctx runbook.Context, name string) {
	ctx.(*_context).stage = name
}

func newContext(load *Load, carton carton.Builder,
	isNative bool) *_context {

	ctx := &_context{
		load:   load,
		carton: carton,
	}

	workDir := workDir(carton, isNative)

	// key-value for each carton's context
	ctx.kv.Init2("context", map[string]interface{}{
		"WORKDIR":  workDir,
		"ISNATIVE": isNative,

		"PN":   carton.Provider(), // PN: provider name
		"T":    filepath.Join(workDir, "temp"),
		"D":    filepath.Join(workDir, "image"),    // install destination directory
		"PKGD": filepath.Join(workDir, "packages"), // points to directory for files to be packaged

		"TARGETARCH":   getTargetArch(carton, isNative),
		"TARGETOS":     getTargetOS(carton, isNative),
		"TARGETVENDOR": getTargetVendor(carton, isNative),
	})

	if dir := carton.SrcDir(workDir); dir != "" {
		ctx.kv.Set("S", dir)
	}
	return ctx
}

func (ctx *_context) mkdir() {

	os.MkdirAll(ctx.kv.GetStr("WORKDIR"), 0755)
	os.MkdirAll(ctx.kv.GetStr("T"), 0755)
	os.MkdirAll(ctx.kv.GetStr("D"), 0755)
	os.MkdirAll(ctx.kv.GetStr("PKGD"), 0755)
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

func (ctx *_context) Wait(upper runbook.Context, runbook, stage string,
	notifier runbook.Notifer) <-chan struct{} {
	return ctx.load.wait(runbook, stage, upper.Get("ISNATIVE").(bool), notifier)
}

func (ctx *_context) Output() (stdout, stderr io.Writer) {

	stdout, stderr = ctx.pool.stdout, ctx.pool.stderr

	if ctx.stage == "" {
		return
	}

	s := ctx.load.getStage(ctx.carton.Provider(), ctx.stage, ctx.Get("ISNATIVE").(bool))

	_stdout, _stderr := s.getIO()

	if _stderr != nil {
		stderr = io.MultiWriter(stderr, _stderr)
	}

	if _stdout != nil {
		stdout = io.MultiWriter(stdout, _stdout)
	}
	return
}

func (ctx *_context) Set(key string, value interface{}) {
	ctx.kv.Set(key, value)
}

// it supports to expand ${[^$]*}
func (ctx *_context) GetStr(key string) string {
	if v := ctx.Get(key); v != nil {
		if v, ok := v.(string); ok {

			// expand ${[^$]*}
			m := regexp.MustCompile(`\${[^$]*}`)
			expand := v
			for _, matched := range m.FindAllString(v, -1) {

				key := matched[2 : len(matched)-1]
				if v := ctx.Get(key); v != nil {

					if v, ok := v.(string); ok {
						expand = m.ReplaceAllString(expand, v)
					}
				} else {

					panic(fmt.Sprintf("GetStr failed to expand key %s from context", key))
				}
			}
			return expand
		}

		return ""
	}

	return ""
}

// retrieves value from context instance first, carton, then golbal settings
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

// return SRC dir & build dir
func (ctx *_context) Dir() (string, string) {

	src := ctx.carton.SrcDir(ctx.GetStr("WORKDIR"))
	build := src

	if b := ctx.GetStr("B"); b != "" {

		if !filepath.IsAbs(b) {
			build = filepath.Join(src, b)
		}
		if utils.IsExist(src) {
			os.Mkdir(build, 0755)
		}
	}
	return src, build
}

func (ctx *_context) Staged(name string) bool {

	runbook := ctx.carton.Provider()

	if ok := ctx.load.isStageLoaded(runbook, name,
		ctx.Get("ISNATIVE").(bool)); ok {
		return true
	}

	if isStagePlayed(runbook, name, ctx.GetStr("T")) {
		return true
	}

	return false
}

// retrieves timeout from carton firstly then golbal settings
func (ctx *_context) Timeout() int {
	if t := ctx.carton.Get("TIMEOUT"); t != nil {
		return t.(int)
	}
	return ctx.load.kv.Get("TIMEOUT").(int)
}

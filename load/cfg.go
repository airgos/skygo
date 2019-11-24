// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package load

import (
	"os"
	"path/filepath"
	"runtime"

	"skygo/runbook"
)

// global variable name
const (
	BUILDIR   = "BUILDIR"
	DLDIR     = "DLDIR"
	TMPDIR    = "TMPDIR"
	BASEWKDIR = "BASEWKDIR"

	IMAGEDIR = "IMAGEDIR"

	// native/building machine's attributes
	NATIVEARCH   = "NATIVEARCH"
	NATIVEOS     = "NATIVEOS"
	NATIVEVENDOR = "NATIVEVENDOR"

	// target machine's attributes
	MACHINE       = "MACHINE"
	MACHINEARCH   = "MACHINEARCH"
	MACHINEOS     = "MACHINEOS"
	MACHINEVENDOR = "MACHINEVENDOR"

	// their value are calcaulated dynamically
	TARGETARCH   = "TARGETARCH"
	TARGETOS     = "TARGETOS"
	TARGETVENDOR = "TARGETVENDOR"
)

var defaultVars = map[string]interface{}{
	NATIVEARCH:   runtime.GOARCH,
	NATIVEOS:     runtime.GOOS,
	NATIVEVENDOR: "",

	MACHINEOS:     "linux",
	MACHINEARCH:   "",
	MACHINEVENDOR: "",

	"TIMEOUT": "1800", // unit is second, default is 30min
}

func getVar(key string) string {
	return defaultVars[key].(string)
}

func loadDefaultCfg(kv *runbook.KV) {

	wd, _ := os.Getwd()

	// default: build
	build := filepath.Join(wd, "build")
	defaultVars[BUILDIR] = build

	// default: build/tmp
	tmp := filepath.Join(build, "tmp")
	defaultVars[TMPDIR] = tmp

	// default: build/tmp/work/
	work := filepath.Join(tmp, "work")
	defaultVars[BASEWKDIR] = work

	// default: build/tmp/deploy/image
	image := filepath.Join(tmp, "deploy", "image")
	defaultVars[IMAGEDIR] = image

	// default: build/downloads
	dl := filepath.Join(build, "downloads")
	defaultVars[DLDIR] = dl

	kv.Init2("loader", defaultVars)
}

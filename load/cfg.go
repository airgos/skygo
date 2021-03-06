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

	MAXLOADERS = "MAXLOADERS" // the number of loader
	TIMEOUT    = "TIMEOUT"    // default timeout for each stage
)

var defaultVars = map[string]interface{}{
	NATIVEARCH:   runtime.GOARCH,
	NATIVEOS:     runtime.GOOS,
	NATIVEVENDOR: "",

	MACHINEOS:     "linux",
	MACHINEARCH:   "",
	MACHINEVENDOR: "",

	TIMEOUT:    600, // unit is second, default is 10min
	MAXLOADERS: 2 * runtime.NumCPU(),
}

var settings *runbook.KV

func getVar(key string) string {
	return defaultVars[key].(string)
}

func init() {

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

	kv := &runbook.KV{}
	kv.Init2("init", defaultVars)
	settings = kv
}

// Settings retrieves global configuration KV
// Global configuration items:
//
//  BUILDIR: top build dir
//  DLDIR: where to save source code archive. default value is BUILDIR/downloads
//  TMPDIR: default is BUILDIR/tmp
//  BASEWKDIR: default value is TMPDIR/work
//  IMAGEDIR: where to store final images. default value is TMPDIR/deploy/image
//  MACHINE: it should be configed outside
//  MACHINEARCH:  it should be configed outside
//  MACHINEOS: default value is linux
//  MACHINEVENDOR: it should be configed outside
//  TARGETARCH: ARCH for specific carton
//  TARGETOS: OS for specific carton
//  TARGETVENDOR: vendor for specific carton
//  TIMEOUT: timeout to build carton. default value is 1800. unit is second
//
func Settings() *runbook.KV {
	return settings
}

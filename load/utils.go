// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package load

import (
	"os"
	"path/filepath"

	"merge/carton"
	"merge/config"
	"merge/log"
)

// WorkDir calculates WORKDIR for carton
// one carton has different WORKDIR for different arch
func WorkDir(c carton.Builder, isNative bool) string {
	dir := getTargetArch(c, isNative)

	if vendor := getTargetVendor(c, isNative); vendor != "" {
		dir = dir + "-" + vendor
	}
	if os := getTargetOS(c, isNative); os != "" {
		dir = dir + "-" + os
	}

	_, ver := c.Resource().Selected()
	pn := c.Provider()
	if isNative {
		pn = pn + "-native"
	}
	dir = filepath.Join(config.GetVar(config.BASEWKDIR), dir, pn, ver)
	dir, _ = filepath.Abs(dir)
	return dir
}

func getTargetArch(c carton.Builder, isNative bool) string {

	if isNative {
		return config.GetVar(config.NATIVEARCH)
	}

	arch, ok := c.LookupVar(config.TARGETARCH)
	if !ok {
		if arch = config.GetVar(config.MACHINEARCH); arch == "" {
			log.Error("MACHINEARCH is not set")
		}
	}
	return arch
}

func getTargetOS(c carton.Builder, isNative bool) string {

	if isNative {
		return config.GetVar(config.NATIVEOS)
	}

	return config.GetVar(config.MACHINEOS)
}

func getTargetVendor(c carton.Builder, isNative bool) string {

	if isNative {
		return config.GetVar(config.NATIVEVENDOR)
	}

	return config.GetVar(config.MACHINEVENDOR)
}

// value of var S
func tempDir(c carton.Builder, isNative bool) string {
	wd := WorkDir(c, isNative)
	return filepath.Join(wd, "temp")
}

func isStagePlayed(stage string, tempDir string) bool {

	done := filepath.Join(tempDir, stage+".done")

	if _, err := os.Stat(done); err == nil {
		log.Trace("%s had been played. Skip it!", stage)
		return true
	}
	return false
}

func markStagePlayed(stage string, tempDir string, played bool) {

	done := filepath.Join(tempDir, stage+".done")
	if played {
		if _, err := os.Create(done); err == nil {
			log.Trace("Mark stage %s to be executed", stage)
		}
		return
	}
	os.Remove(done)
}

// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package load

import (
	"os"
	"path/filepath"

	"skygo/carton"
	"skygo/utils"
	"skygo/utils/log"
)

// workDir calculates WORKDIR for carton
// one carton has different WORKDIR for different arch
func workDir(c carton.Builder, isNative bool) string {
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
	dir = filepath.Join(getVar(BASEWKDIR), dir, pn, ver)
	dir, _ = filepath.Abs(dir)
	return dir
}

func getTargetArch(c carton.Builder, isNative bool) string {

	if isNative {
		return getVar(NATIVEARCH)
	}

	arch := c.Get(TARGETARCH)
	if arch == nil {
		if arch = getVar(MACHINEARCH); arch == "" {
			log.Error("MACHINEARCH is not set")
		}
	}
	return arch.(string)
}

func getTargetOS(c carton.Builder, isNative bool) string {

	if isNative {
		return getVar(NATIVEOS)
	}

	return getVar(MACHINEOS)
}

func getTargetVendor(c carton.Builder, isNative bool) string {

	if isNative {
		return getVar(NATIVEVENDOR)
	}

	return getVar(MACHINEVENDOR)
}

// value of var S
func tempDir(c carton.Builder, isNative bool) string {
	wd := workDir(c, isNative)
	return filepath.Join(wd, "temp")
}

func isStagePlayed(carton, stage string, tempDir string) bool {

	done := filepath.Join(tempDir, stage+".done")

	if utils.IsExist(done) {
		log.Trace("%s's stage %s had been played. Skip it!", carton, stage)
		return true
	}
	return false
}

func markStagePlayed(carton, stage string, tempDir string, played bool) {

	done := filepath.Join(tempDir, stage+".done")
	if played {
		if _, err := os.Create(done); err == nil {
			log.Trace("Mark %s's stage %s to be executed", carton, stage)
		}
		return
	}
	os.Remove(done)
}

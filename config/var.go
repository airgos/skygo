package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// global variable name
const (
	TOPDIR    = "TOPDIR"
	BUILDIR   = "BUILDIR"
	DLDIR     = "DLDIR"
	TMPDIR    = "TMPDIR"
	BASEWKDIR = "BASEWKDIR"

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

// vars hosts values
type vars map[string]string

var defaultVars = vars{
	NATIVEARCH:   runtime.GOARCH,
	NATIVEOS:     runtime.GOOS,
	NATIVEVENDOR: "",

	MACHINEOS:     "linux",
	MACHINEARCH:   "",
	MACHINEVENDOR: "",
}

// GetVar return value of var key
func GetVar(key string) string {
	return defaultVars[key]
}

// SetVar return value of var key
func SetVar(key, value string) {
	defaultVars[key] = value
}

// LookupVar retrieves the value of the variable named by the key.
// If the variable is present, value (which may be empty) is returned
// and the boolean is true. Otherwise the returned value will be empty
// and the boolean will be false.
func LookupVar(key string) (string, bool) {
	value, ok := defaultVars[key]
	return value, ok
}

func init() {
	wd, _ := os.Getwd()
	SetVar(TOPDIR, wd)

	// default: build
	build := filepath.Join(wd, "build")
	SetVar(BUILDIR, build)

	// default: build/tmp
	tmp := filepath.Join(build, "tmp")
	SetVar(TMPDIR, tmp)

	// default: build/tmp/work/
	work := filepath.Join(tmp, "work")
	SetVar(BASEWKDIR, work)

	// default: build/downloads
	dl := filepath.Join(build, "downloads")
	SetVar(DLDIR, dl)
}

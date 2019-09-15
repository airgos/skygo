package config

import (
	"os"
	"path/filepath"
)

type vars map[string]string

var defaultVars vars = make(vars)

func NewVars() vars {
	return make(vars)
}

// GetVar return value of var key
func (v vars) GetVar(key string) string {
	return v[key]
}

// SetVar return value of var key
func (v vars) SetVar(key, value string) {
	v[key] = value
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

const (
	TOPDIR  = "TOPDIR"
	BUILDIR = "BUILDIR"
	DLDIR   = "DLDIR"
	TMPDIR  = "TMPDIR"
	WORKDIR = "WORKDIR"

	SRCDIR = "SRCDIR"
)

func init() {
	wd, _ := os.Getwd()
	defaultVars.SetVar(TOPDIR, wd)

	// default: build
	build := filepath.Join(wd, "build")
	defaultVars.SetVar(BUILDIR, build)

	// default: build/tmp
	tmp := filepath.Join(build, "tmp")
	defaultVars.SetVar(TMPDIR, tmp)

	// default: build/tmp/work/
	work := filepath.Join(tmp, "work")
	defaultVars.SetVar(WORKDIR, work)

	// default: build/downloads
	dl := filepath.Join(build, "downloads")
	defaultVars.SetVar(DLDIR, dl)
}

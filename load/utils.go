package load

import (
	"path/filepath"
	"strings"

	"merge/carton"
	"merge/config"
	"merge/log"
)

// WorkDir calculates WORKDIR for carton
// one carton has different WORKDIR for different arch
func WorkDir(c carton.Builder, isNative bool) string {
	var dir string

	if isNative {
		dir = strings.Join([]string{config.GetVar(config.NATIVEARCH),
			config.GetVar(config.NATIVEOS)}, "-")
	} else {

		arch, ok := c.LookupVar(config.MACHINEARCH)
		if !ok {
			arch = config.GetVar(config.MACHINEARCH)
		}

		if arch == "" {
			log.Error("MACHINEARCH is not set")
		}

		dir = strings.Join([]string{arch,
			config.GetVar(config.MACHINEOS)}, "-")
	}
	_, ver := c.Resource().Selected()
	dir = filepath.Join(config.GetVar(config.BASEWKDIR), dir, c.Provider(), ver)
	dir, _ = filepath.Abs(dir)
	return dir
}

package config

import "os"

// arch, http thread
// arch, soc, machine -> arm1176-js, Puma6, CGNM-2250

func DownloadDir() string {
	dir := "build/download"
	os.MkdirAll(dir, 0755)
	return dir
}

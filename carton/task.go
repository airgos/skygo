package carton

import (
	"context"
	"fmt"
	"merge/config"
	"merge/fetch"
	"merge/runbook"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FetchAndExtract fetch resource given by b.SrcURL to download dir, then
// copy or extract to WorkPath
func FetchAndExtract(ctx context.Context, b Builder) error {

	f := fetch.NewFetch(b.SrcURL(), config.DownloadDir(), b)
	for {
		e, ok := f()
		if e != nil {
			return e
		}
		if !ok {
			break
		}
	}

	return nil
}

// Patch search patch/diff files under WorkPath, sort, then apply
func Patch(ctx context.Context, b Builder) error {

	wd := b.WorkPath()
	file, e := os.Open(wd)
	if e != nil {
		return nil
	}
	fpaths, e := file.Readdirnames(-1)
	if e != nil {
		return nil
	}
	sort.Strings(fpaths)
	for _, fpath := range fpaths {

		// TODO: log
		if strings.HasSuffix(fpath, ".diff") || strings.HasSuffix(fpath, ".patch") {
			patch := filepath.Join(wd, fpath)
			fmt.Printf("Apply patch file %s\n", patch)
			if e := runbook.PatchFile(ctx, patch, b); e != nil {
				return e
			}
		}
	}
	return nil
}

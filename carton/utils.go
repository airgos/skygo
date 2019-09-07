// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package carton

import (
	"context"
	"fmt"
	"merge/log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

var patchcmd = `
[ -d .git ] && {
	git am --committer-date-is-author-date $PATCHFILE
	exit $?
}

git init
git config  user.email "robot@$(hostname)"
git config  user.name "robot"
git add -A
git commit -m 'first commit'

git apply $PATCHFILE && {
	git add -A
	git commit -m "apply patch: $(basename $PATCHFILE)"
}
`

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
		select {
		case <-ctx.Done():
		default:

			if strings.HasSuffix(fpath, ".diff") || strings.HasSuffix(fpath, ".patch") {

				log.Trace("To apply patch %s, fpath")

				patch := filepath.Join(wd, fpath)

				cmd := exec.CommandContext(ctx, "/bin/bash", "-c", patchcmd)
				cmd.Dir = b.SrcPath()
				cmd.Stdout, cmd.Stderr = b.Output()

				cmd.Env = append(cmd.Env, b.Environ()...)
				cmd.Env = append(cmd.Env, fmt.Sprintf("PATCHFILE=%s\n", patch))

				if e := cmd.Run(); e != nil {
					return fmt.Errorf("patch: %s", e)
				}

			}
		}
	}
	return nil
}

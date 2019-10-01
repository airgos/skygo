// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package load

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"merge/log"
	"merge/runbook"
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

// patch search patch/diff files under WorkPath, sort, then apply
func patch(ctx context.Context) error {

	arg, _ := runbook.FromContext(ctx)

	file, e := os.Open(arg.Wd)
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

				patch := filepath.Join(arg.Wd, fpath)

				cmd := exec.CommandContext(ctx, "/bin/bash", "-c", patchcmd)
				cmd.Dir = arg.SrcDir(arg.Wd)
				cmd.Stdout, cmd.Stderr = arg.Output()

				cmd.Env = append(cmd.Env, fmt.Sprintf("PATCHFILE=%s\n", patch))

				if e := cmd.Run(); e != nil {
					return fmt.Errorf("patch: %s", e)
				}

			}
		}
	}
	return nil
}

// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package load

import (
	"context"
	"fmt"
	"os"
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

				log.Trace("To apply patch %s", fpath)
				command := runbook.NewCommand(ctx, "/bin/bash", "-c", patchcmd)

				patch := filepath.Join(arg.Wd, fpath)
				command.Cmd.Env = append(command.Cmd.Env, fmt.Sprintf("PATCHFILE=%s\n", patch))
				if e := command.Run("patch"); e != nil {
					return e
				}
			}
		}
	}
	return nil
}

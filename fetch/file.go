// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fetch

import (
	"context"
	"os"
	"path/filepath"

	"merge/fetch/utils"
	"merge/runbook"
)

func file(ctx context.Context, url string) error {

	arg, _ := runbook.FromContext(ctx)
	wd := arg.Direnv.WorkPath()

	// skip file://
	url = url[7:]
	for _, d := range arg.Direnv.FilePath() {

		path := filepath.Join(d, url)
		fileinfo, err := os.Stat(path)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		target := filepath.Join(wd, filepath.Base(url))
		utils.CopyFile(target, fileinfo.Mode(), file)
		// TODO: copy when mod time and content is chagned
		break
	}
	return nil
}

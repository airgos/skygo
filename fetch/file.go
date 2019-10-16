// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fetch

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"merge/fetch/utils"
	"merge/runbook"
	"merge/runbook/xsync"
)

func file(ctx context.Context, url string, updated *bool) error {

	arg, _ := runbook.FromContext(ctx)
	stdout, _ := arg.Output()

	url = url[7:]
	for _, u := range arg.FilesPath {

		from := filepath.Join(u, url)
		if _, err := os.Stat(from); err != nil {
			continue // not found
		}

		target := filepath.Join(arg.Wd, url)
		os.MkdirAll(filepath.Dir(target), 0755)
		changed, err := copyFile(ctx, target, from, stdout)
		if changed {
			*updated = true
		}
		return err
	}

	return fmt.Errorf("%s is not found in FilesPath", url)
}

func copyFile(ctx context.Context, to, from string, stdout io.Writer) (bool, error) {

	file, err := os.Open(from)
	if err != nil {
		return false, err
	}
	defer file.Close()
	fileinfo, _ := file.Stat()

	if _, err := os.Stat(to); err != nil {
		fmt.Fprintf(stdout, "Copy %s to %s\n", from, to)
		return true, utils.CopyFile(to, fileinfo.Mode(), file)
	}

	var sum1, sum2 [md5.Size]byte
	g, ctx := xsync.WithContext(ctx)
	g.Go(func() error {
		data, err := ioutil.ReadFile(from)
		if err != nil {
			return err
		}
		sum1 = md5.Sum(data)
		return nil
	})

	g.Go(func() error {
		data, err := ioutil.ReadFile(to)
		if err != nil {
			return err
		}
		sum2 = md5.Sum(data)
		return nil
	})
	if err := g.Wait(); err != nil {
		return false, err
	}

	if !bytes.Equal(sum1[:], sum2[:]) {

		fmt.Fprintf(stdout, "Sync %s to %s\n", from, to)
		return true, utils.CopyFile(to, fileinfo.Mode(), file)
	}
	return false, nil
}

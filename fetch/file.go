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
	"runtime"
	"strings"

	"skygo/runbook"
	"skygo/runbook/xsync"
	"skygo/utils"
)

type fileSync struct {
	from, to string
}

func file(ctx context.Context, url string, notify func(bool)) error {

	arg := runbook.FromContext(ctx)
	stdout, _ := arg.Output()

	url = url[7:]
	for _, u := range arg.FilesPath {

		root := filepath.Join(u, url)
		if utils.IsExist(root) {

			g, ctx := xsync.WithContext(ctx)
			paths := make(chan fileSync)

			g.Go(func() error {

				defer close(paths)
				return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}

					rel := strings.TrimPrefix(path, u)                  // remove prefix dir of FilesPath
					target := filepath.Join(arg.GetStr("WORKDIR"), rel) // full target path

					if info.IsDir() {
						return os.MkdirAll(target, 0755)
					}

					if info.Mode().IsRegular() {

						select {
						case paths <- fileSync{
							from: path,
							to:   target,
						}:
						case <-ctx.Done():
							return ctx.Err()
						}
						return nil
					} else {
						link, err := os.Readlink(path)
						if err == nil {
							os.Symlink(link, target)
						}
						return err
					}
				})
			})

			for i := 0; i < runtime.NumCPU(); i++ {
				g.Go(func() error {
					for files := range paths {
						updated, err := copyFile(ctx, files.to, files.from, stdout)
						if err != nil {
							return err
						}
						notify(updated)
					}
					return nil
				})
			}
			err := g.Wait()
			return err
		}
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

	if !utils.IsExist(to) {
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

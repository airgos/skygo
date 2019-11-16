// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package utils

import (
	"os"
	"path/filepath"
	"strings"

	"skygo/utils/log"
)

// StageBox represents state of stage
// It stage file/dir in while list but exlcude file/dir in black list
type StageBox struct {
	whiteList []string
	blackList []string
}

// Push adds file or directory @path to while list
func (s *StageBox) Push(path string) *StageBox {
	if filepath.IsAbs(path) {
		panic("StageBox rejects ABS path")
	}
	s.whiteList = append(s.whiteList, path)
	return s
}

// Pop adds file or directory @path to blank list
func (s *StageBox) Pop(path string) *StageBox {
	if filepath.IsAbs(path) {
		panic("StageBox rejects ABS path")
	}
	s.blackList = append(s.blackList, path)
	return s
}

// Stage creates hard links to copies of available files under from into directory to
func (s *StageBox) Stage(from, to string) error {

	if err := s.stage(from, to); err != nil {
		return err
	}

	// remove dir/files in blacklist
	for _, b := range s.blackList {
		t := filepath.Join(to, b)
		log.Trace("Remove %s\n", t)
		os.RemoveAll(t)
	}
	return nil
}

// creates hard links to copies of the white list under from into directory to
// A hard Link:
//   can’t cross the file system boundaries (i.e. A hardlink can only work on the same filesystem),
//   can’t link directories,
//   has the same inode number and permissions of original file,
//   permissions will be updated if we change the permissions of source file,
//   has the actual contents of original file, so that you still can view the contents, even if the original file moved or removed.
func (s *StageBox) stage(from, to string) error {

	for _, w := range s.whiteList {

		root := filepath.Join(from, w)
		if _, err := os.Stat(root); err != nil {
			continue
		}

		if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			rel := strings.TrimPrefix(path, from)
			if info.Mode().IsRegular() {
				dest := filepath.Join(to, rel)
				log.Trace("Staging hark link:\n\t%s -->\n\t%s\n", dest, path)
				os.MkdirAll(filepath.Dir(dest), 0755)
				return os.Link(path, dest)
			} else {
				link, err := os.Readlink(path)
				if err == nil { // symbol link
					os.Symlink(link, filepath.Join(to, rel))
				}
				return err
			}
		}); err != nil {
			return err
		}
	}
	return nil
}

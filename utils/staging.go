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

	// stage white list
	for _, w := range s.whiteList {
		if err := Stage(filepath.Join(from, w),
			filepath.Join(to, w)); err != nil {
			return err
		}
	}
	// remove dir/files in blacklist
	for _, b := range s.blackList {
		t := filepath.Join(to, b)
		log.Trace("Remove %s\n", t)
		os.RemoveAll(t)
	}
	return nil
}

// copy symbol link or create hard link
func stageFile(from, to string, info os.FileInfo) error {

	if info.Mode()&os.ModeSymlink != 0 {
		from, err := os.Readlink(from)
		if err != nil {
			return err
		}
		log.Trace("Copy symbol link: %s --> %s\n", from, to)
		return os.Symlink(from, to)
	}

	srcinfo, _ := os.Stat(filepath.Dir(from))
	os.MkdirAll(filepath.Dir(to), srcinfo.Mode())

	log.Trace("Create hark link:\n\t%s -->\n\t%s\n", from, to)
	return os.Link(from, to)
}

// Stage creates hard links recursively
// ignore if source @from does not exists
// A hard Link:
//   can’t cross the file system boundaries (i.e. A hardlink can only work on the same filesystem),
//   can’t link directories,
//   has the same inode number and permissions of original file,
//   permissions will be updated if we change the permissions of source file,
//   has the actual contents of original file, even if the original file moved or removed.
func Stage(from, to string) error {

	if info, err := os.Stat(from); err == nil && info.IsDir() {
		log.Info("Staging recursively:\n\t%s -->\n\t%s\n", from, to)
	} else if os.IsNotExist(err) {
		return nil
	}

	// another way: use ioutil.ReadDir. which is faster ?
	if err := filepath.Walk(from, func(path string, info os.FileInfo, err error) error {
		var dest string

		if err != nil {
			return err
		}

		if from == path {
			dest = to
			if info.IsDir() {
				os.MkdirAll(from, info.Mode())
				return nil
			}
		} else {

			rel := strings.TrimPrefix(path, from)
			if info.IsDir() {
				os.MkdirAll(filepath.Join(to, rel), info.Mode())
				return nil
			}

			dest = filepath.Join(to, rel)
		}
		return stageFile(path, dest, info)
	}); err != nil {
		return err
	}

	return nil
}

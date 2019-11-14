// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CopyFile read from IO r and write to file @name with FileMode @mode
func CopyFile(name string, mode os.FileMode, r io.Reader) error {

	os.MkdirAll(filepath.Dir(name), 0755)
	w, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer w.Close()
	if _, err := io.Copy(w, r); err != nil {
		return err
	}
	return nil
}

// CreateSymbolicLink create symbol link
func CreateSymbolicLink(fpath string, linkName string) error {
	err := os.MkdirAll(filepath.Dir(fpath), 0755)
	if err != nil {
		return fmt.Errorf("%s: mkdir: %v", fpath, err)
	}

	err = os.Symlink(linkName, fpath)
	if err != nil {
		return fmt.Errorf("%s: making symbolic link: %v", fpath, err)
	}

	return nil
}

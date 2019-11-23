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
func CreateSymbolicLink(filePath string, linkName string) error {
	err := os.MkdirAll(filepath.Dir(filePath), 0755)
	if err != nil {
		return fmt.Errorf("%s: mkdir: %v", filePath, err)
	}

	err = os.Symlink(linkName, filePath)
	if err != nil {
		return fmt.Errorf("%s: making symbolic link: %v", filePath, err)
	}

	return nil
}

// IsExist returns a boolean indicating whether the filePath(a file or
// directory) already exists
func IsExist(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}

	return true
}

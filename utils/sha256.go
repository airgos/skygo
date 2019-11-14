// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// Sha256Matched return whether checksum is matched
// if not matched, sha256 checksum is returned
func Sha256Matched(csum, fpath string) (bool, string) {

	file, _ := os.Open(fpath)
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return false, ""
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if sum == csum {
		return true, ""
	}
	return false, sum
}

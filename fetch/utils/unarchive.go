// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package utils

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Unarchiver is the interface to extract archiver
type Unarchiver interface {
	Unarchive(fpath, dest string) error
}

var unarchiver = map[string]Unarchiver{
	".zip":     unzip,
	".tar":     untar,
	".tar.gz":  untgz,
	".tgz":     untgz,
	".tbz2":    untbz2,
	".tar.bz2": untbz2,
}

// NewUnarchive create new Unarchiver
func NewUnarchive(fpath string) Unarchiver {
	for k, u := range unarchiver {
		if strings.HasSuffix(fpath, k) {
			// fmt.Printf("Unarchiver %s is selected!\n", k)
			return u
		}
	}
	return nil
}

type zipfmt struct{}

var unzip zipfmt

func (zipfmt) Unarchive(fpath, dest string) error {
	r, e := zip.OpenReader(fpath)
	if e != nil {
		return e
	}
	defer r.Close()
	for _, zf := range r.File {

		// fmt.Printf("file:%s\n", zf.Name)
		f, err := zf.Open()
		if err != nil {
			return fmt.Errorf("%s: open compressed file: %v", zf.Name, err)
		}
		defer f.Close()
		fpath := filepath.Join(dest, zf.Name)
		if e := CopyFile(fpath, zf.FileInfo().Mode(), f); e != nil {
			return e
		}
	}
	return nil
}

type tarfmt struct{}

var untar tarfmt

func (tarfmt) Unarchive(fpath, dest string) error {
	file, e := os.Open(fpath)
	if e != nil {
		return e
	}
	defer file.Close()
	tr := tar.NewReader(file)
	return unTar(tr, dest)
}

func unTar(tr *tar.Reader, dest string) error {

	for {
		header, err := tr.Next()

		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dest, header.Name)
		// fmt.Println(target)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("mkdir %s:%v", target, err)
			}
		case tar.TypeReg, tar.TypeRegA, tar.TypeChar, tar.TypeBlock, tar.TypeFifo:
			CopyFile(target, header.FileInfo().Mode(), tr)
			os.Chtimes(target, header.AccessTime, header.ModTime)
		case tar.TypeSymlink:
			CreateSymbolicLink(target, header.Linkname)

		default:
			return fmt.Errorf("%s: Unknown Typeflag", header.Name)
		}
	}
	return nil
}

type tgzfmt struct{}

var untgz tgzfmt

func (tgzfmt) Unarchive(fpath, dest string) error {
	file, e := os.Open(fpath)
	if e != nil {
		return nil
	}
	defer file.Close()
	gr, e := gzip.NewReader(file)
	if e != nil {
		return e
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	return unTar(tr, dest)
}

type tbz2fmt struct{}

var untbz2 tbz2fmt

func (tbz2fmt) Unarchive(fpath, dest string) error {
	file, e := os.Open(fpath)
	if e != nil {
		return nil
	}
	defer file.Close()

	br := bzip2.NewReader(file)
	tr := tar.NewReader(br)
	return unTar(tr, dest)
}

// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"merge/fetch/utils"
	"merge/runbook"
	"merge/runbook/xsync"
)

//TODO:
// don't unpack again if it's done

// support scheme http and https. if file is archiver, unpack it
func httpAndUnpack(ctx context.Context, dd string, url string) error {
	arg, _ := runbook.FromContext(ctx)
	stdout, _ := arg.Output()

	slice := strings.Split(url, "#")
	if len(slice) != 2 {
		return fmt.Errorf("%s - URL[%s] have no checksum", arg.Owner, url)
	}
	u := slice[0]

	base := filepath.Base(u)
	fpath := filepath.Join(dd, base)

	fmt.Fprintf(stdout, "To download %s\n", u)
	if e := download(ctx, u, slice[1], fpath); e != nil {
		return e
	}

	if unar := utils.NewUnarchive(fpath); unar != nil {
		fmt.Fprintf(stdout, "unarchive %s\n", fpath)
		if e := unar.Unarchive(fpath, arg.Wd); e != nil {
			return fmt.Errorf("unarchive %s failed:%s", base, e.Error())
		}
	}
	return nil
}

func download(ctx context.Context, url, checksum, fpath string) error {

	done := fpath + ".done"
	if _, e := os.Stat(done); e == nil {
		return nil
	}
	r, e := http.Head(url)
	if e != nil {
		return e
	}
	defer r.Body.Close()

	h := r.Header
	a := h.Get("Accept-Ranges")
	l := h.Get("Content-Length")

	length, _ := strconv.Atoi(l)
	// don't fetch in parallel if file size is less then 0.5M=0.5*1024*1024
	if a != "" && length > 524288 {
		fetchInParallel(ctx, fpath, url, length)
	} else {
		return fetchSlice(ctx, 0, 0, url, fpath)
	}

	if ok, sum := utils.Sha256Matched(checksum, fpath); !ok {
		return fmt.Errorf("ErrCheckSum: %s %s", fpath, sum)
	}

	os.Create(done)
	return nil
}

func fetchSlice(ctx context.Context, start, stop int, url, fpath string) error {

	client := http.Client{}
	req, e := http.NewRequestWithContext(ctx, "GET", url, nil)
	if e != nil {
		return e
	}
	if stop > 1 {
		req.Header.Add("Range", fmt.Sprintf("bytes=%d-%d", start, stop-1))
	}

	r, e := client.Do(req)
	if e != nil {
		return e
	}
	defer r.Body.Close()

	if e = utils.CopyFile(fpath, 0664, r.Body); e != nil {
		return e
	}
	return nil
}

func fetchInParallel(ctx context.Context, fpath, url string, length int) error {
	var e error

	connections := runtime.NumCPU()
	slices := make([]string, connections)

	sub := length / connections
	diff := length % connections
	g, ctx := xsync.WithContext(ctx)

	for i := 0; i < connections; i++ {
		slice := fmt.Sprintf("%s.%d", fpath, i)
		slices[i] = slice

		start := sub * i
		stop := start + sub
		if i == connections-1 {
			stop += diff
		}
		g.Go(func() error {
			return fetchSlice(ctx, start, stop, url, slice)
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	// merge file
	files := make([]io.Reader, connections)
	for i := 0; i < connections; i++ {
		if files[i], e = os.Open(slices[i]); e != nil {
			return e
		}
	}

	r := io.MultiReader(files...)
	if e = utils.CopyFile(fpath, 0664, r); e != nil {
		return e
	}

	for i := 0; i < connections; i++ {
		os.Remove(slices[i])
	}

	return nil
}

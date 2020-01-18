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

	"skygo/runbook"
	"skygo/runbook/xsync"
	"skygo/utils"
	"skygo/utils/unarchive"
)

//TODO:
// don't unpack again if it's done

// support scheme http and https. if file is archiver, unpack it
func httpAndUnpack(ctx context.Context, url string,
	httpGet func(ctx context.Context, from, to string) error,
	notify func(bool)) error {

	arg := runbook.FromContext(ctx)
	stdout, _ := arg.Output()

	slice := strings.Split(url, "#")
	if len(slice) != 2 {
		return fmt.Errorf("%s - URL[%s] have no checksum", arg.Owner, url)
	}

	from := slice[0]
	checksum := slice[1]
	dldir, _ := arg.LookupVar("DLDIR")
	to := filepath.Join(dldir, filepath.Base(from))

	done := to + ".done"
	if !utils.IsExist(done) {

		// TODO: if found in mirror, replace with mirror URL
		fmt.Fprintf(stdout, "To download %s\n", from)
		if httpGet != nil {
			os.Remove(to)
			if err := httpGet(ctx, from, to); err != nil {
				return err
			}
		} else {
			if err := builtinGet(ctx, from, to); err != nil {
				return err
			}
		}

		if ok, sum := utils.Sha256Matched(checksum, to); !ok {
			return fmt.Errorf("ErrCheckSum: %s %s", to, sum)
		}
		os.Create(done)
	}

	if unar := unarchive.NewUnarchive(to); unar != nil {
		fmt.Fprintf(stdout, "unarchive %s\n", to)
		if e := unar.Unarchive(to, arg.GetStr("WORKDIR")); e != nil {
			return fmt.Errorf("unarchive %s failed:%s", to, e.Error())
		}
	}
	return nil
}

func builtinGet(ctx context.Context, from, to string) error {

	r, e := http.Head(from)
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
		return fetchInParallel(ctx, to, from, length)
	}
	return fetchSlice(ctx, 0, 0, from, to)
}

func fetchSlice(ctx context.Context, start, stop int, url, to string) error {

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

	if e = utils.CopyFile(to, 0664, r.Body); e != nil {
		return e
	}
	return nil
}

func fetchInParallel(ctx context.Context, to, url string, length int) error {
	var e error

	connections := runtime.NumCPU()
	slices := make([]string, connections)

	sub := length / connections
	diff := length % connections
	g, ctx := xsync.WithContext(ctx)

	for i := 0; i < connections; i++ {
		slice := fmt.Sprintf("%s.%d", to, i)
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

	// skygo file
	files := make([]io.Reader, connections)
	for i := 0; i < connections; i++ {
		if files[i], e = os.Open(slices[i]); e != nil {
			return e
		}
	}

	r := io.MultiReader(files...)
	if e = utils.CopyFile(to, 0664, r); e != nil {
		return e
	}

	for i := 0; i < connections; i++ {
		os.Remove(slices[i])
	}

	return nil
}

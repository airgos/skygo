package fetch

import (
	"errors"
	"fmt"
	"io"
	"merge/fetch/utils"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

//TODO:
// get fetch thread
// don't fetch in parallel in file size is less then 0.5M
// download: use type int64
// trace log

type httpFetch struct{}

// HTTPS https scheme fetcher
var HTTPS httpFetch

func (httpFetch) Fetch(url, dest string, f Fetcher) error {

	slice := strings.Split(url, "#")
	if len(slice) != 2 {
		return errors.New("URL have no checksum")
	}
	u := slice[0]

	base := filepath.Base(u)
	fpath := filepath.Join(dest, base)

	if e := download(u, slice[1], fpath); e != nil {
		return e
	}

	if unar := utils.NewUnarchive(fpath); unar != nil {
		fmt.Printf("unarchive %s\n", fpath)
		if e := unar.Unarchive(fpath, f.WorkPath()); e != nil {
			return fmt.Errorf("unarchive %s failed:%s", base, e.Error())
		}
	}
	return nil
}

func download(url, checksum, fpath string) error {

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
	if a != "" && l != "" {
		length, _ := strconv.Atoi(l)
		fetchInParallel(fpath, url, length)
	} else {
		r, e := http.Get(url)
		if e != nil {
			return e
		}
		defer r.Body.Close()
		e = utils.CopyFile(fpath, 0664, r.Body)
		if e != nil {
			return e
		}
	}

	if ok, sum := utils.Sha256Matched(checksum, fpath); !ok {
		return fmt.Errorf("ErrCheckSum: %s %s, but expect %s", fpath, sum, checksum)
	}

	os.Create(done)
	return nil
}

func fetchSlice(start, stop int, url, fpath string) error {

	client := http.Client{}
	req, e := http.NewRequest("GET", url, nil)
	if e != nil {
		return e
	}
	req.Header.Add("Range", fmt.Sprintf("bytes=%d-%d", start, stop-1))

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

func fetchInParallel(fpath, url string, length int) error {
	var wg sync.WaitGroup
	var e error

	connections := runtime.NumCPU()
	slices := make([]string, connections)
	wg.Add(connections)

	sub := length / connections
	diff := length % connections

	errc := make(chan error, connections)

	for i := 0; i < connections; i++ {
		slice := fmt.Sprintf("%s.%d", fpath, i)
		slices[i] = slice

		start := sub * i
		stop := start + sub
		if i == connections-1 {
			stop += diff
		}
		go func(start, end int, url, fpath string) {
			if e = fetchSlice(start, stop, url, fpath); e != nil {
				errc <- e
			}
			wg.Done()
		}(start, stop, url, slice)
	}
	wg.Wait()

	if len(errc) != 0 {
		if e = <-errc; e != nil {
			return e
		}
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

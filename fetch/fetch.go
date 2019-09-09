// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fetch

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"merge/log"
	"merge/runbook"
)

// Resource represent state of fetch
type Resource struct {
	dd string // download path.absolute directory path

	resource map[string]SrcURL

	// preferred version
	// print error log if prefer version is set again
	prefer string
	m      sync.Mutex
	done   uint32

	selected string // indicated which version is selected

	// save only one url failed to fetch
	failedURL string
	once      sync.Once
}

// SrcURL holds a collection of Source URL in specific version
type SrcURL struct {
	head *list.List
}

type fetchCmd struct {
	fetch interface{}
	url   string
}

func (cmd *fetchCmd) Download(ctx context.Context, res *Resource) error {

	switch m := cmd.fetch.(type) {

	// scheme file: handler
	case func(context.Context, string) error:
		return m(ctx, cmd.url)

	// for http,https,vscGit
	case func(context.Context, string, string) error:
		return m(ctx, res.dd, cmd.url)

	default:
		return errors.New("Unknown fetch command")
	}
}

// NewFetch create fetch state
func NewFetch(dd string) *Resource {

	fetch := new(Resource)

	fetch.dd = dd
	fetch.resource = make(map[string]SrcURL)
	return fetch
}

// ByVersion get SrcURL by version
// If not found, create empty holder
func (fetch *Resource) ByVersion(version string) *SrcURL {

	if res, ok := fetch.resource[version]; ok {
		return &res
	}
	res := SrcURL{head: list.New()}
	fetch.resource[version] = res
	return &res
}

// Versions sort all SrcURL from latest to older, then return in slice
func (fetch *Resource) Versions() []string {

	num := len(fetch.resource)
	versions := make([]string, num)
	i := 0
	for v := range fetch.resource {
		versions[i] = v
		i++
	}

	min := func(x, y int) int {
		if x < y {
			return x
		}
		return y
	}
	// example version sorting result: 2.0 > 1.0.1 > 1.0 > HEAD
	sort.Slice(versions, func(i, j int) bool {

		a := strings.Split(versions[i], ".")
		b := strings.Split(versions[j], ".")
		num := min(len(a), len(b))
		for i := 0; i < num; i++ {
			na, e := strconv.Atoi(a[i])
			if e != nil {
				return false
			}
			nb, _ := strconv.Atoi(b[i])
			if na > nb {
				return true
			}

			if na < nb {
				return false
			}
		}
		return len(a) > len(b)
	})
	return versions
}

// Prefer set preferred version of SrcURL
func (fetch *Resource) Prefer(version string) {

	if atomic.LoadUint32(&fetch.done) == 1 {
		log.Warning("Try to set preferred version again!")
		return
	}

	fetch.m.Lock()
	defer fetch.m.Unlock()
	if fetch.done == 0 {
		defer atomic.StoreUint32(&fetch.done, 1)
		fetch.prefer = version
	}
}

// Selected return selected SrcURL and its version
// select preferred then latest version of SrcURL
func (fetch *Resource) Selected() (*SrcURL, string) {

	if fetch.selected == "" {

		if fetch.prefer != "" {
			fetch.selected = fetch.prefer
		} else {
			versions := fetch.Versions()
			if len(versions) > 0 {
				fetch.selected = versions[0]
			}
		}
	}

	if res, ok := fetch.resource[fetch.selected]; ok {
		return &res, fetch.selected
	}
	return nil, ""
}

// Download download all source URL held by selected SrcURL
// Extract automatically if source URL is an archiver, like tar.bz2
func (fetch *Resource) Download(ctx context.Context) error {

	var wg sync.WaitGroup
	arg, _ := runbook.FromContext(ctx)

	res, _ := fetch.Selected()
	if res == nil {
		log.Warning("%s don't hold any source URL", arg.Owner)
		return nil
	}

	h := res.head

	ctx, cancel := context.WithCancel(ctx)
	wg.Add(h.Len())
	for e := h.Front(); e != nil; e = e.Next() {
		go func(e *list.Element) {

			fetchCmd := e.Value.(*fetchCmd)
			if err := fetchCmd.Download(ctx, fetch); err != nil {
				fetch.once.Do(func() {
					fetch.failedURL = fetchCmd.url
					cancel()
				})
			}
			wg.Done()
		}(e)
	}
	wg.Wait()
	if fetch.failedURL != "" {
		return fmt.Errorf("failed to fetch %s", fetch.failedURL)
	}
	return nil
}

// PushFile push scheme file:// to SrcURL
// srcurl can hold multiple URL with delimeter space
func (res *SrcURL) PushFile(srcurl string) *SrcURL {

	url := strings.Fields(srcurl)
	for _, u := range url {

		url := fetchCmd{
			fetch: file,
			url:   u,
		}
		res.head.PushBack(&url)
	}
	return res
}

// PushVcs push one vcs repository to SrcURL
// srcurl is repository or repository@revision
// repository must be known by vcs utility like git
// revision identifier for the underlying source repository, such as a commit
// hash prefix, revision tag, or branch name, selects that specific code revision.
// valid srcurl example:
//     https://github.com:foo/bar.git
//     https://github.com:foo/bar.git@v1.1
//     https://github.com:foo/bar.git@c198403
func (res *SrcURL) PushVcs(srcurl string) *SrcURL {

	if strings.Contains(srcurl, " ") {
		// TODO: who ?
		panic("it contains multiple repo in one url")
	}

	url := fetchCmd{
		fetch: vcsFetch,
		url:   srcurl,
	}
	res.head.PushBack(&url)
	return res
}

// PushHTTP push Http or Https URL to SrcURL
// srcurl's scheme must be https:// or http://, and sha256 checksum must be
// append at the end with delimeter #
// e.g.  http://x.y.z/foo.tar.bz2#sha256
// srcurl can hold multiple URL with delimeter space
func (res *SrcURL) PushHTTP(srcurl string) *SrcURL {

	url := strings.Fields(srcurl)
	for _, u := range url {

		url := fetchCmd{
			fetch: httpAndUnpack,
			url:   u,
		}
		res.head.PushBack(&url)
	}
	return res
}

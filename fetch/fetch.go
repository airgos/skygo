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
	"merge/runbook/xsync"
)

// Resource represent state of fetch
type Resource struct {
	resource map[string]SrcURL

	// preferred version
	// print error log if prefer version is set again
	prefer string
	m      sync.Mutex
	done   uint32

	selected string // indicated which version is selected
}

// SrcURL holds a collection of Source URL in specific version
type SrcURL struct {
	head *list.List
}

type fetchCmd struct {
	fetch interface{}
	url   string
}

// Download grab url in Resource
// notify tell whether source code change is detected
func (cmd *fetchCmd) Download(ctx context.Context, res *Resource,
	notify func(bool)) error {

	url := strings.TrimSpace(cmd.url)
	switch m := cmd.fetch.(type) {

	case func(context.Context, string, func(bool)) error:
		return m(ctx, url, notify)

	default:
		return errors.New("Unknown fetch command")
	}
}

// NewFetch create fetch state
func NewFetch() *Resource {

	fetch := new(Resource)
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
// if source code is updated, it calls notify
func (fetch *Resource) Download(ctx context.Context,
	notify func(ctx context.Context)) error {

	arg, _ := runbook.FromContext(ctx)

	res, _ := fetch.Selected()
	if res == nil {
		log.Warning("%s don't hold any source URL", arg.Owner)
		return nil
	}
	log.Trace("Start downloading source URLs owned by %s", arg.Owner)

	h := res.head

	var once sync.Once
	g, ctx := xsync.WithContext(ctx)
	for e := h.Front(); e != nil; e = e.Next() {
		e := e // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {

			fetchCmd := e.Value.(*fetchCmd)
			if err := fetchCmd.Download(ctx, fetch, func(updated bool) {
				if notify != nil && updated {
					once.Do(func() { notify(ctx) })
				}
			}); err != nil {
				return fmt.Errorf("failed to fetch %s. Reason: \n\t %s", fetchCmd.url, err)
			}
			return nil
		})
	}

	return g.Wait()
}

// Push push source URL srcurl to SrcURL
// srcurl can hold multiple URL with delimeter space
// Push try to detect scheme by order:
//  file://            find locally under FilesPath
//  vcs, pls refer to PushVcs
//  http:// https://   grab from network
func (src *SrcURL) Push(srcurl string) *SrcURL {

	url := strings.Fields(srcurl)
	for _, u := range url {
		if strings.HasPrefix(u, "file://") {
			src.pushFile(u)
			continue
		}

		if bySuffix(u) != nil {
			src.PushVcs(u)
			continue
		}

		if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
			src.PushHTTP(u, nil)
			continue
		}
		panic(fmt.Sprintf("Unknown source URL: %s", u))
	}
	return src
}

// Pushfile push one scheme file:// to SrcURL
func (src *SrcURL) pushFile(srcurl string) *SrcURL {

	url := fetchCmd{
		fetch: file,
		url:   srcurl,
	}
	src.head.PushBack(&url)
	return src
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
// Mostly, Push can push vcs repository URL, reserved this API for fallback
func (src *SrcURL) PushVcs(srcurl string) *SrcURL {

	if strings.Contains(srcurl, " ") {
		panic(fmt.Sprintf("repository %s has SPACE", srcurl))
	}

	url := fetchCmd{
		fetch: vcsFetch,
		url:   srcurl,
	}
	src.head.PushBack(&url)
	return src
}

// PushHTTP push Http or Https URL to SrcURL
// srcurl's scheme must be https:// or http://, and sha256 checksum must be
// append at the end with delimeter #
// e.g.  http://x.y.z/foo.tar.bz2#sha256
//
// httpGet is the caller own get function, it's optional(value is nil).
// httpGet does not need to handle checksum, since parameter from does not
// contain checksum. Example implementation:
// func wget(ctx context.Context, from, to string) error {

// 	arg := "-t 2 -T 30 -nv --no-check-certificate"
// 	args := strings.Fields(fmt.Sprintf("%s %s", arg, from))

// 	cmd := runbook.NewCommand(ctx, "wget", args...)
// 	cmd.Cmd.Dir = filepath.Dir(to)
// 	if err := cmd.Cmd.Run(); err != nil {
// 		return err
// 	}
// 	return nil
// }
func (src *SrcURL) PushHTTP(srcurl string,
	httpGet func(ctx context.Context, from, to string) error) *SrcURL {

	url := fetchCmd{
		fetch: func(ctx context.Context, url string, notify func(bool)) error {
			return httpAndUnpack(ctx, url, httpGet, notify)
		},
		url: srcurl,
	}
	src.head.PushBack(&url)
	return src
}

package fetch

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// Resource represent state of fetch
type Resource struct {
	resource map[string]SrcURL
	dd, wd   string   // absolute directory path
	filepath []string // search dir list for scheme file://

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

func (cmd *fetchCmd) Download(ctx context.Context, res *Resource) error {

	switch m := cmd.fetch.(type) {

	// for http,https,vscGit
	case func(context.Context, []string, string, string) error:
		return m(ctx, res.filepath, res.wd, cmd.url)

	// scheme file: handler
	case func(context.Context, string, string, string) error:
		return m(ctx, res.dd, res.wd, cmd.url)

	default:
		return errors.New("Unknown fetch command")
	}
}

// NewFetch create fetch state
func NewFetch(dd, wd string, filepath []string) *Resource {

	fetch := new(Resource)

	fetch.wd = wd
	fetch.dd = dd
	fetch.filepath = filepath
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
		// TODO: log
		fmt.Fprintf(os.Stdout, "Try to set preferred version again!")
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

	res, _ := fetch.Selected()
	if res == nil {
		// TODO: warning not found
		return nil
	}

	h := res.head
	for e := h.Front(); e != nil; e = e.Next() {
		select {
		case <-ctx.Done():
			return fmt.Errorf("Fetch is aborted by context")
		default:
			fetchCmd := e.Value.(*fetchCmd)
			if err := fetchCmd.Download(ctx, fetch); err != nil {
				return err
			}
		}
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

// PushVcs push git repository url to SrcURL
// srcurl's scheme must be known by utility git. tag and commit sha can be
// appened at the end with delimeter @, like git://x.y.z@v1.1
// srcurl can hold multiple URL with delimeter space
func (res *SrcURL) PushVcs(srcurl string) *SrcURL {

	url := strings.Fields(srcurl)
	for _, u := range url {

		url := fetchCmd{
			fetch: vcsGit,
			url:   u,
		}
		res.head.PushBack(&url)
	}
	return res
}

// PushHTTP push Http or Https URL to SrcURL
// srcurl's scheme must be https:// or http://, and sha256 checksum must be
// append at the end with delimeter #
// e.g.  http://x.y.z#sha256
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

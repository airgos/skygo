package fetch

import (
	"fmt"
	"io"
	"strings"
)

// SrcURL represent src URL with additional info, like Sha256 checksum, git revision
// that can be branch name, tag name or the prefix of the commit hash.
// SrcURL can hold multiple src URL with space delimiter.
// If sign @ is in the URL, its protocol is git regardless of scheme
// Example:
// git://x.y.z@v1.1
// http://x.y.z@master
// http://x.y.z#sha256
// https://x.y.z#sha256
// file://path/local/file
type SrcURL string

// Fetcher implement methods to help fetch
type Fetcher interface {
	Output() (stdout, stderr io.Writer)
	WorkPath() string
	FilePath() []string
}

// Handler implement method to fetch
type Handler interface {
	Fetch(url, dest string, f Fetcher) error
}

// map protocol to fetch handler
// TODO: "https://github.com/neovim/neovim.git", it's git protocol, not http
// 1. add match associated with each handler, git before http/https
// 2. global match, allow user to add hook based on domain/protoocl?. Better, it can help download from special http site(extract real url)
var fetchers = map[string]Handler{

	"http":  HTTPS,
	"https": HTTPS,
	"git":   GIT,
	"file":  FILE,
}

// RegisterHandler register scheme handler
func RegisterHandler(scheme string, h Handler) {
	fetchers[scheme] = h
}

// NewFetch create iterator to fetch URLs one by one
func NewFetch(url []SrcURL, dest string, fetcher Fetcher) func() (error, bool) {

	index := 0
	size := len(url)

	return func() (error, bool) {
		var scheme string

		if index >= size {
			return nil, false
		}
		s := string(url[index])

		if i := strings.Index(s, "://"); i > 0 {
			scheme = s[0:i]

		} else {

			return fmt.Errorf("Fetch: invalid URL %s", s), false
		}

		// regardless of scheme, it's considered as git protocol
		if strings.Contains(s, "@") {
			scheme = "git"
		}

		f, ok := fetchers[scheme]
		if !ok {
			return fmt.Errorf("Fetch: no handler for scheme %s", scheme), false
		}
		index++

		return f.Fetch(s, dest, fetcher), true
	}
}

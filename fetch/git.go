package fetch

import (
	"fmt"
)

// ssh --> git@github.com:neovim/neovim.git
// https --> https://github.com/neovim/neovim.git
// git --> git://git.busybox.net/busybox
// if contains .git before #, it's git. else check scheme: file or https, http

type gitFetch struct{}

// GIT git scheme fetcher
var GIT gitFetch

func (gitFetch) Fetch(url, dest string, f Fetcher) error {
	fmt.Println("GIT Fetch no implemented")
	return nil
}

// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fetch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"merge/runbook"
)

// inspired by go/src/cmd/go/internal/get/vcs.go

type vcsCmd struct {
	cmd string
	dir string
	ctx context.Context
	env map[string]string

	index string
	repo  string
	tag   string

	createCmd   []string
	downloadCmd []string

	tagLookUpCmd   []tagCmd
	tagSyncCmd     []string
	tagSyncDefault []string
	tagNewCmd      string

	// used to indentify which kind of vcs
	//pingCmd
}

type tagCmd struct {
	cmd     string
	pattern string // used to match result of cmd
}

var vcsGit = vcsCmd{

	cmd:       "git",
	index:     ".git",
	createCmd: []string{"clone $repo"},

	// tag is either tag name or branch name
	tagLookUpCmd: []tagCmd{
		{"show-ref tags/$tag origin/$tag", `((?:tags|origin)/\S+)$`},
	},

	tagSyncCmd:     []string{"checkout $tag"},
	tagSyncDefault: []string{"checkout master"},

	// used to create pesudo tag if rev is not branch name or tag name
	tagNewCmd: "tag $tag $tag",
}

func byRepo(repo, tag string) *vcsCmd {

	// TODO: support another vcs
	// match kind of vcs based on postfix, like .git
	// match kind of vcs based on domain, e.g. match github.com to git
	// invoke method ping to detect kind of vcs. method ping relies on pingCmd
	vcs := &vcsGit

	vcs.tag = tag
	vcs.repo = repo
	vcs.env = map[string]string{
		"$repo": repo,
		"$tag":  tag,
	}

	return vcs
}

func (vcs *vcsCmd) run(dir, cmdline string) ([]byte, error) {

	var buf bytes.Buffer
	args := strings.Fields(cmdline)
	for j, arg := range args {
		k := arg
		if i := strings.LastIndex(arg, "/"); i > 0 {
			k = arg[i+1:]
		}
		if v, ok := vcs.env[k]; ok {
			args[j] = strings.ReplaceAll(arg, k, v)
		}
	}
	// fmt.Println(vcs.cmd, args)
	cmd := exec.CommandContext(vcs.ctx, vcs.cmd, args...)
	cmd.Dir = dir

	arg, _ := runbook.FromContext(vcs.ctx)
	stdout, stderr := arg.Output()
	cmd.Stdout, cmd.Stderr =
		io.MultiWriter(stdout, &buf),
		io.MultiWriter(stderr, &buf)

	if e := cmd.Run(); e != nil {
		return nil, fmt.Errorf("Failed to run %s %s", vcs.cmd, cmdline)
	}
	return buf.Bytes(), nil
}

// look up repo, if not found, create it
func (vcs *vcsCmd) lookupRepo(wd string) error {

	path := vcs.repo
	if i := strings.Index(vcs.repo, "//"); i >= 0 {
		path = vcs.repo[i+2:] // skip //
	}

	if i := strings.Index(path, vcs.index); i >= 0 {
		path = path[:i]
	}

	vcs.dir = filepath.Join(wd, filepath.Base(path))
	if _, err := os.Stat(filepath.Join(vcs.dir, vcs.index)); err != nil && os.IsNotExist(err) {
		dir := filepath.Dir(vcs.dir)
		for _, cmd := range vcs.createCmd {
			if _, e := vcs.run(dir, cmd); e != nil {
				return e
			}
		}
	}
	return nil
}

func (vcs *vcsCmd) syncTag() error {

	var tagSyncCmd []string

	if vcs.tag != "" {

		tag := ""
		for _, tc := range vcs.tagLookUpCmd {
			out, e := vcs.run(vcs.dir, tc.cmd)
			if e != nil {
				break
			}
			re := regexp.MustCompile(`(?m-s)` + tc.pattern)
			m := re.FindStringSubmatch(string(out))
			if len(m) > 1 {
				tag = m[1]
				break
			}
		}

		// create pesudo tag
		if tag == "" {
			if _, e := vcs.run(vcs.dir, vcs.tagNewCmd); e != nil {
				return e
			}
		}

		tagSyncCmd = vcs.tagSyncCmd
	} else {

		tagSyncCmd = vcs.tagSyncDefault
	}

	for _, cmd := range tagSyncCmd {
		if _, e := vcs.run(vcs.dir, cmd); e != nil {
			return e
		}
	}
	return nil
}

func vcsFetch(ctx context.Context, dd string, url string,
	notify func(bool)) error {

	arg, _ := runbook.FromContext(ctx)
	repo := url
	tag := ""

	if i := strings.LastIndex(url, "@"); i >= 0 {
		repo, tag = url[:i], url[i+1:]
	}

	vcs := byRepo(repo, tag)
	vcs.ctx = ctx

	if e := vcs.lookupRepo(arg.Wd); e != nil {
		return e
	}
	if e := vcs.syncTag(); e != nil {
		return e
	}

	return nil
}

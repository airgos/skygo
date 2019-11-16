// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fetch

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"skygo/runbook"
)

// inspired by go/src/cmd/go/internal/get/vcs.go

type vcsCmd struct {
	cmd string
	dir string
	ctx context.Context
	env map[string]string

	index string
	// used to indentify which kind of vcs
	pattern string

	repo string
	tag  string

	createCmd   []string
	downloadCmd []string

	tagLookUpCmd   []tagCmd
	tagSyncCmd     []string
	tagSyncDefault []string
	tagNewCmd      string

	revCmd string

	//TODO:
	//add field pingCmd to check kind on the fly
}

type tagCmd struct {
	cmd     string
	pattern string // used to match result of cmd
}

var vcsGit = vcsCmd{

	cmd:   "git",
	index: ".git",

	// example matched url a.b.c/x.git, a.b.c/x.git@a234
	pattern: `((?:\.git@.+|\.git))$`,

	createCmd: []string{"clone $repo"},

	// tag is either tag name or branch name
	tagLookUpCmd: []tagCmd{
		{"show-ref tags/$tag origin/$tag", `((?:tags|origin)/\S+)$`},
	},

	tagSyncCmd:     []string{"checkout $tag"},
	tagSyncDefault: []string{"checkout master"},

	// used to create pesudo tag if rev is not branch name or tag name
	tagNewCmd: "tag $tag $tag",

	revCmd: "rev-parse HEAD",
}

var vcsList = []*vcsCmd{&vcsGit}

func byRepo(repo, tag string) *vcsCmd {

	// TODO: support another vcs
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
	command := runbook.NewCommand(vcs.ctx, vcs.cmd, args...)
	command.Cmd.Dir = dir

	arg := runbook.FromContext(vcs.ctx)
	stdout, stderr := arg.Output()
	command.Cmd.Stdout, command.Cmd.Stderr =
		io.MultiWriter(stdout, &buf),
		io.MultiWriter(stderr, &buf)

	if e := command.Run("fetch"); e != nil {
		return nil, e
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
	index := filepath.Join(vcs.dir, vcs.index)
	dir := filepath.Dir(vcs.dir)
	if _, err := os.Stat(index); err != nil && os.IsNotExist(err) {
		for _, cmd := range vcs.createCmd {
			if _, e := vcs.run(dir, cmd); e != nil {
				return e
			}
		}
	} else {
		// index is invalid, to create repo again
		if _, err = vcs.run(vcs.dir, vcs.revCmd); err != nil {
			os.RemoveAll(vcs.dir)
			for _, cmd := range vcs.createCmd {
				if _, e := vcs.run(dir, cmd); e != nil {
					return e
				}
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

func vcsFetch(ctx context.Context, url string,
	notify func(bool)) (err error) {

	var rev1, rev2 []byte

	arg := runbook.FromContext(ctx)
	repo := url
	tag := ""

	if i := strings.LastIndex(url, "@"); i >= 0 {
		repo, tag = url[:i], url[i+1:]
	}

	vcs := byRepo(repo, tag)
	vcs.ctx = ctx

	if e := vcs.lookupRepo(arg.GetVar("WORKDIR")); e != nil {
		return e
	}

	// get revision before syncTag
	if rev1, err = vcs.run(vcs.dir, vcs.revCmd); err != nil {
		return err
	}

	if err = vcs.syncTag(); err != nil {
		return err
	}

	// get revision after syncTag
	if rev2, err = vcs.run(vcs.dir, vcs.revCmd); err != nil {
		return err
	}

	notify(!bytes.Equal(rev1, rev2))
	return nil
}

func bySuffix(url string) *vcsCmd {

	for _, vcs := range vcsList {
		re := regexp.MustCompile(vcs.pattern)
		if re.MatchString(url) {
			return vcs
		}
	}
	return nil
}

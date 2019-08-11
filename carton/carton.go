// Package carton implements ...

// Dependency Format: carton-name[@procedure]

package carton

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"boxgo/config"
	"boxgo/fetch"
	"boxgo/runbook"
)

// Error used by carton
var (
	ErrNotFound = errors.New("Not Found")
	ErrNoName   = errors.New("Illegal Provider")
	ErrAbsPath  = errors.New("Abs Path")
)

// predefined stage
const (
	FETCH   = "fetch"
	PATCH   = "patch"
	PREPARE = "prepare"
	BUILD   = "build"
	INSTALL = "install"
	TEST    = "test"
)

// The Carton represents the state of carton
// It implements interface Builder and Modifier
type Carton struct {
	Name     string
	provider []string
	Desc     string // oneline description
	Homepage string // home page

	stdout, stderr io.Writer

	file     []string // which files offer this carton
	srcpath  string   // path(dir) of SRC code
	filepath []string // search dirs for scheme file://

	depends      []string // needed for both running and building
	buildDepends []string // only needed when building from scratch

	resouce map[string][]fetch.SrcURL // a collection of src URL
	prefer  string                    // prefer version of resource

	// environment variables who are exported to cartion running space by format key=value
	environ map[string]string

	runbook *runbook.Runbook

	m      sync.Mutex
	inited uint32
}

// Init must make sure Carton is inited only once time
// file: which file provide this carton
func (c *Carton) Init() {

	if atomic.LoadUint32(&c.inited) == 1 {
		return
	}

	c.m.Lock()
	defer c.m.Unlock()
	if c.inited == 0 {
		defer atomic.StoreUint32(&c.inited, 1)

		c.provider = []string{}
		c.environ = make(map[string]string)
		if c.Name != "" {
			c.provider = append(c.provider, c.Name)
			c.environ["PN"] = c.Name
		}

		c.file = []string{}
		c.filepath = []string{}

		c.resouce = make(map[string][]fetch.SrcURL)
	}
}

// InstallRunbook install default runbook
func (c *Carton) InstallRunbook() {

	chain := runbook.NewRunbook(c)
	p, _ := chain.PushFront(FETCH).AddTask(0, func() error {
		return fetchExtract(c)
	})
	p, _ = p.InsertAfter(PATCH).AddTask(0, func() error {
		return patch(c)
	})
	p.InsertAfter(PREPARE).InsertAfter(BUILD).InsertAfter(INSTALL)
	c.runbook = chain
}

// Output return io.Writer Stdout, Stderr
func (c *Carton) Output() (stdout, stderr io.Writer) {
	return c.stdout, c.stderr
}

// SetOutput set Stdout, Stderr
func (c *Carton) SetOutput(stdout, stderr io.Writer) {
	c.stdout = stdout
	c.stderr = stderr
}

// Provider return what's provided
func (c *Carton) Provider() string {
	return c.Name
}

// From add new location indicating which file provide carton
// Return location list
func (c *Carton) From(file ...string) []string {

	notAdded := func(from string) bool {
		for _, f := range c.file {
			if f == from {
				return false
			}
		}
		return true
	}

	if len(file) != 0 {

		if from := file[0]; from != "" {

			if notAdded(from) {
				c.file = append(c.file, from)
				filepath := strings.TrimSuffix(from, ".go")
				c.filepath = append(c.filepath, filepath)
			}
		}
	}

	return c.file
}

// BuildDepends add depends only required for building from scratch
// Always return the same kind of depends
func (c *Carton) BuildDepends(dep ...string) []string {

	if len(dep) == 0 {
		return c.buildDepends
	}
	c.buildDepends = append(c.buildDepends, dep...)
	return c.buildDepends
}

// Depends add depends required for building from scratch, running or both
// Always return the same kind of depends
func (c *Carton) Depends(dep ...string) []string {

	if len(dep) == 0 {
		return c.depends
	}
	c.depends = append(c.depends, dep...)
	return c.depends
}

// SrcPath give under which source code is
func (c *Carton) SrcPath() string {

	if c.srcpath != "" {

		return c.srcpath
	}

	if file, e := os.Open(c.WorkPath()); e == nil {
		var d string
		if fpaths, e := file.Readdirnames(-1); e == nil {

			// choose the only one dir
			if len(fpaths) == 1 {
				d = filepath.Join(c.WorkPath(), fpaths[0])
				if info, e := os.Stat(d); e == nil && info.IsDir() {
					c.srcpath = d
					return d
				}
			}

			if ver := c.version(); ver == "HEAD" || ver == "" {
				d = c.Provider()
			} else {
				d = fmt.Sprintf("%s-%s", c.Provider(), ver)
			}
			d = filepath.Join(c.WorkPath(), d)
			c.srcpath = d
			return d
		}
	}
	return ""
}

// SetSrcPath set SrcPath explicitily. It joins with output of WorkPath() as SrcPath
func (c *Carton) SetSrcPath(dir string) error {
	if filepath.IsAbs(dir) {
		return ErrAbsPath
	}
	c.srcpath = filepath.Join(c.WorkPath(), dir)
	return nil
}

// AddFilePath appends one dir path
// based on who(carton provider) call, don't give full path
func (c *Carton) AddFilePath(dir string) error {

	// TODO: find dir in file provider dir
	c.filepath = append(c.filepath, dir)
	return nil
}

// FilePath return FilePath
func (c *Carton) FilePath() []string {
	return c.filepath
}

// AddSrcURL add SrcURL which is a set of source URL. Each URL is delimited by SPACE
// version: used to identify which SrcURL. If URL's protocol is git, use "HEAD" as version
func (c *Carton) AddSrcURL(version string, srcURL string) {
	url := strings.Fields(srcURL)
	for _, u := range url {
		c.resouce[version] = append(c.resouce[version], fetch.SrcURL(u))
	}
}

// AddHeadSrc add git repository URLs with delimiter SPACE
func (c *Carton) AddHeadSrc(srcURL string) {

	c.resouce["HEAD"] = append(c.resouce["HEAD"], fetch.SrcURL(srcURL))
}

// SrcURL get the latest version of source URL
// Use preferred version first if it's set
// A return value of nil indicates no SrcURL
func (c *Carton) SrcURL() []fetch.SrcURL {

	if c.prefer != "" {
		return c.resouce[c.prefer]
	}

	return c.resouce[c.Versions()[0]]
}

// Versions give available version as slice. HEAD is pushed back
// A return nil indicates no SrcURL is added
func (c *Carton) Versions() []string {

	if len(c.resouce) == 0 {
		return nil
	}

	version := []string{}
	for k := range c.resouce {
		version = append(version, k)
	}

	min := func(x, y int) int {
		if x < y {
			return x
		}
		return y
	}
	sort.Slice(version, func(i, j int) bool {

		a := strings.Split(version[i], ".")
		b := strings.Split(version[j], ".")
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
		}
		return len(a) > len(b)
	})
	return version
}

// version return which version of SrcURL will be selected
func (c *Carton) version() string {
	if c.prefer != "" {
		return c.prefer
	}
	if v := c.Versions(); v != nil {
		return v[0]
	}
	return ""
}

// PreferSrcURL let user to decide which version of srcURL is used
func (c *Carton) PreferSrcURL(version string) {
	// TODO: insure version exist
	c.prefer = version
}

// WorkPath return value of WorkPath
func (c *Carton) WorkPath() string {

	dir := fmt.Sprintf("%s", c.Provider())
	// TODO: get from config package
	dir = filepath.Join("build", dir)
	dir, _ = filepath.Abs(dir)
	if _, e := os.Stat(dir); e != nil {
		os.MkdirAll(dir, 0755)
	}
	return dir
}

// CloneRunbook clone runbook with different runtime
func (c *Carton) CloneRunbook(r runbook.Runtime) *runbook.Runbook {
	return c.runbook.Clone(r)
}

// Perform carry out all stages owned
// Break if any stage failed
func (c *Carton) Perform() error {
	return c.runbook.Perform()
}

// Play run stage's task or independent task
func (c *Carton) Play(name string) error {
	if s := c.runbook.Stage(name); s != nil {
		if e := s.Play(); e != nil {
			return e
		}
	}
	return c.runbook.RunTask(name)
}

// Stage get stage from the runbook
func (c *Carton) Stage(name string) *runbook.Stage {
	return c.runbook.Stage(name)
}

// RunbookInfo give stage slice with the number of task, independent task names
func (c *Carton) RunbookInfo() ([]string, []int, []string) {
	return c.runbook.RunbookInfo()
}

// AddJob collects routine/function from @name, map to stage with the same
// routine name, then add task with weight 0 to runbook
// name: either raw string or file name under WorkPath
// if not match, add to independent task
func (c *Carton) AddJob(name string) {
	// TODO: implement
}

// TaskSet get independent TaskSet with each other
func (c *Carton) TaskSet() *runbook.TaskSet {
	return c.runbook.TaskSet()
}

// Environ returns a copy of strings representing the environment,
// in the form "key=value".
func (c *Carton) Environ() []string {
	env := make([]string, 0, len(c.environ))
	for k, v := range c.environ {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	env = append(env, fmt.Sprintf("PV=%s", c.version()), fmt.Sprintf("SRC=%s", c.SrcPath()))
	return env
}

// Setenv sets the value of the environment variable named by the key.
// It returns an error, if any.
func (c *Carton) Setenv(key, value string) {
	c.environ[key] = value
}

func (c *Carton) String() string {

	var b strings.Builder

	if c.Desc != "" {
		fmt.Fprintf(&b, "%s\n", c.Desc)
	}

	if c.Homepage != "" {
		fmt.Fprintf(&b, "%s\n", c.Homepage)
	}

	if len(c.provider) > 0 {
		fmt.Fprintf(&b, "Provider: %s", c.provider[0])
		for _, p := range c.provider[1:] {
			fmt.Fprintf(&b, " %s", p)
		}
		fmt.Fprintf(&b, "\n")
	}

	// where come from
	fmt.Fprintf(&b, "From: %s\n", c.file[0])
	for _, file := range c.file[1:] {
		fmt.Fprintf(&b, "      %s\n", file)
	}

	return b.String()
}

func fetchExtract(c *Carton) error {

	f := fetch.NewFetch(c.SrcURL(), config.DownloadDir(), c)
	for {
		e, ok := f()
		if e != nil {
			return e
		}
		if !ok {
			break
		}
	}

	return nil
}

// search patch/diff files under WorkPath, sort, then apply
func patch(c *Carton) error {

	wd := c.WorkPath()
	file, e := os.Open(wd)
	if e != nil {
		return nil
	}
	fpaths, e := file.Readdirnames(-1)
	if e != nil {
		return nil
	}
	sort.Strings(fpaths)
	for _, fpath := range fpaths {

		// TODO: log
		if strings.HasSuffix(fpath, ".diff") || strings.HasSuffix(fpath, ".patch") {
			patch := filepath.Join(wd, fpath)
			fmt.Printf("Apply patch file %s\n", patch)
			if e := runbook.PatchFile(patch, c); e != nil {
				return e
			}
		}
	}
	return nil
}

package app

import (
	"boxgo/carton"
	"flag"
	"fmt"
)

type info struct {
}

func (*info) Name() string         { return "info" }
func (*info) Usage() string        { return "<carton name>" }
func (*info) Summary() string      { return "show information of carton" }
func (*info) Help(f *flag.FlagSet) {}

func (*info) Run(args ...string) error {
	if len(args) == 0 {
		return commandLineErrorf("carton name must be supplied")
	}

	c, virtual := carton.Find(args[0])
	if c == nil {
		return fmt.Errorf("%s not found", args[0])
	}

	if virtual {
		fmt.Printf("%s --> %s\n\n", args[0], c.Provider())
	}

	show(c)

	return nil
}

func show(h carton.Builder) {

	// TODO: indicates whether it is installed, and version
	fmt.Printf("%s", h.Provider())
	versions := h.Versions()
	HEAD := false
	if len(versions) > 0 {
		fmt.Printf(": %s", versions[0])
		for _, ver := range versions[1:] {
			fmt.Printf(", %s", ver)
		}
		HEAD = versions[len(versions)-1] == "HEAD"
	}
	fmt.Println()
	fmt.Println(h)

	if HEAD {
		fmt.Println("==> Options")
		fmt.Printf("%s\n %s\n", "--HEAD", "Install HEAD version")
	}

	// print dependencies
	builds := h.BuildDepends()
	depends := h.Depends()
	if len(builds) > 0 || len(depends) > 0 {
		fmt.Println("==> Dependencies")
		if len(builds) > 0 {

			fmt.Printf("Build: %s", builds[0])
			for _, d := range builds[1:] {
				fmt.Printf(", %s", d)
			}
			fmt.Println()
		}

		if len(depends) > 0 {

			fmt.Printf("Required: %s", depends[0])
			for _, d := range depends[1:] {
				fmt.Printf(", %s", d)
			}
			fmt.Println()
		}
	}

	fmt.Println("==> Path")
	fmt.Println("Work:", h.WorkPath())
	fmt.Println("Src: ", h.SrcPath())

	filepath := h.FilePath()
	fmt.Println("File:", filepath[0])
	for _, p := range filepath[1:] {
		fmt.Println("     ", p)
	}

	fmt.Println("==> Runbook")
	stage, tasknum, taskname := h.Runbook().RunbookInfo()
	fmt.Printf("Stage: %s[%d]", stage[0], tasknum[0])
	for i := 1; i < len(stage); i++ {
		fmt.Printf(" --> %s[%d]", stage[i], tasknum[i])
	}
	if len(taskname) > 0 {
		fmt.Printf("\nTasks: %s", taskname[0])
		for _, n := range taskname[1:] {
			fmt.Printf(" %s", n)
		}
	}
}

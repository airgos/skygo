package carton

import (
	"boxgo/runbook"
	"fmt"
	"strings"
)

// Image inherits Carton
type Image struct {
	Name string
	Desc string
	Type string
	Carton
}

// Provider return what's provided
func (e *Image) Provider() string {
	return e.Name
}

func (e *Image) String() string {

	var b strings.Builder

	if e.Desc != "" {

		fmt.Fprintf(&b, "%s\n", e.Desc)
	}
	fmt.Fprintf(&b, "%s\n", e.Carton.String())
	return b.String()
}

// InstallRunbook install default runbook
func (e *Image) InstallRunbook() {

	chain := runbook.NewRunbook(e)
	chain.PushFront(PREPARE).
		InsertAfter(BUILD).
		InsertAfter(INSTALL)
	e.runbook = chain
}

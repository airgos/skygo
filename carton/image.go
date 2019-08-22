package carton

import (
	"merge/runbook"
	"runtime"
)

// Image inherits Carton
type Image struct {
	Type string
	Carton
}

func (e *Image) String() string { return e.Carton.String() }

// NewImage create a image carton and add to inventory
func NewImage(name string, m func(i *Image)) {

	i := new(Image)
	i.name = name
	_, file, _, _ := runtime.Caller(1)

	// inherits i.Carton.Init
	i.Init(file, i, func(arg Modifier) {

		rb := runbook.NewRunbook(i)
		rb.PushFront(PREPARE).
			InsertAfter(BUILD).
			InsertAfter(INSTALL)
		i.RunBook = rb
		m(i)
	})
}

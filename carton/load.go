package carton

import (
	"io"
	"runtime"
	"sync"
)

// Load represent state of load
type Load struct {
	ch     chan *resource
	res    []*resource
	carton string
	works  int
}

type resource struct {
	stdout, stderr io.Writer
}

// NewLoad create load to build carton
// num represent how many loader work. if its value is 0, it will use default value
func NewLoad(num int, carton string) *Load {

	if num == 0 {
		num = runtime.NumCPU()
	}
	load := Load{
		ch:     make(chan *resource, num),
		res:    make([]*resource, num),
		carton: carton,
		works:  num,
	}
	for i := 0; i < num; i++ {
		res := new(resource)
		load.res[i] = res
		load.ch <- res
	}

	return &load
}

func (l *Load) get() *resource {
	return <-l.ch
}

func (l *Load) put(res *resource) {
	l.ch <- res
}

// SetOutput assign stdout & stderr for one load
// It's not safe to invoke during loading
func (l *Load) SetOutput(index int, stdout, stderr io.Writer) *Load {
	l.res[index].stderr = stderr
	l.res[index].stdout = stdout
	return l
}

// TODO: handle error
func (l *Load) run(carton string) {
	var wg sync.WaitGroup

	b, _, _ := Find(carton)
	deps := b.BuildDepends()
	required := b.Depends()
	deps = append(deps, required...)

	wg.Add(len(deps))
	for _, d := range deps {
		go func(carton string) {

			l.run(carton)
			wg.Done()
		}(d)
	}
	wg.Wait()
	res := l.get()
	b.SetOutput(res.stdout, res.stderr)
	b.Runbook().Perform()
	l.put(res)
}

// Run start loading
func (l *Load) Run() {
	l.run(l.carton)
}

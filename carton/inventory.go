package carton

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
)

// hold all carton and its variants
var inventory = make(map[string]Builder)
var virtualInventory = make(map[string]Builder)

var initCh = make(chan func())
var updateCh = make(chan func())

var updateNum int

/*
Carton's dependcy hierarchy is a directed graph. Each carton is vertex,
its dependcy are adjacent edges.

graph coloring algorithm is used to detect loop in the dependcy hierarchy.

White Color: Vertices which are not processed will be assigned white colors.
	So at the beginning all the vertices will be white.
Gray Color: Vertices will are currently being processed. If DFS(Depth-First
	Search) is started from a particular vertex will be in gray color till
	DFS is not completed
Black Color: Vertices for which the DFS is completed, means all the processed
	vertices will be assigned black color.
Cycle Detection: During DFS if we encounter a vertex which is already in Gray
	color (means this vertex already in processing and in Gray color in the
	current DFS) then we have detected a Cycle and edge from current vertex
	to gray vertex will a back edge.

refer:
https://algorithms.tutorialhorizon.com/graph-detect-cycle-in-a-directed-graph-using-colors/
*/
const (
	white = iota
	gray
	black
)

// add carton to inventory
func add(carton Builder, file string, f func()) {

	name := carton.Provider()
	if name == "" {
		panic(fmt.Sprintf("Carton Err:", ErrNoName))
	}

	if _, ok := inventory[name]; ok {
		panic(fmt.Sprintf("Carton Err: %s had been added!", name))
	}
	inventory[name] = carton

	// run in goroutine to improve user experience
	go func() {
		initCh <- func() {
			if f != nil {
				f()
			}
			carton.From(file)
		}
	}()
}

func addVirtual(carton Builder, target, file string) {

	carton.From(file)
	virtualInventory[target] = carton
}

// Update find the carton and then update it in callback
func Update(name string, m func(Modifier)) {

	if carton, ok := inventory[name]; ok && m != nil {

		_, file, _, _ := runtime.Caller(1)
		updateNum++

		// run in goroutine to improve user experience
		go func() {
			updateCh <- func() {
				carton.From(file)
				m(carton.(Modifier))
			}
		}()
	}
}

// Find find the carton by name
// if not found, return ErrNotFound
func Find(name string) (h Builder, isVirtual bool, e error) {

	// TODO: handle if exist in both inventory
	if carton, ok := inventory[name]; ok {
		return carton, false, nil
	}

	if virtual, ok := virtualInventory[name]; ok {
		return virtual, true, nil
	}
	return nil, true, ErrNotFound
}

// BuildInventory build carton warehouse and then check whether each carton has
// loop dependcy hierarchy. if loop is found, it's returned in slice
func BuildInventory(ctx context.Context) (bool, []string) {
	buildInventory()
	return detectLoopDep(ctx)
}

func buildInventory() {

	var wg sync.WaitGroup
	num := runtime.NumCPU()

	wg.Add(len(inventory))
	for i := 0; i < num; i++ {
		go func() {

			for cb := range initCh {
				cb()
				wg.Done()
			}
		}()
	}
	wg.Wait()
	close(initCh)

	wg.Add(updateNum)
	for i := 0; i < num; i++ {
		go func() {
			for cb := range updateCh {
				cb()
				wg.Done()
			}
		}()
	}
	wg.Wait()
	close(updateCh)
}

// detectLoopDep find whether inventory has any carton whose dependcy hierarchy
// has loop. Loop is returned in slice
func detectLoopDep(ctx context.Context) (bool, []string) {

	var (
		wg   sync.WaitGroup
		once sync.Once
		loop []string
	)

	ctx, cancel := context.WithCancel(ctx)

	recv := func() chan string {

		ch := make(chan string)
		go func() {
			for carton := range inventory {
				ch <- carton
			}
			close(ch)
		}()
		return ch
	}()

	detect := func(ctx context.Context, recv chan string) {
		for {
			select {
			case <-ctx.Done():
				return
			case carton := <-recv:
				if carton == "" { //why occur
					return
				}
				if looped, l := hasLoopDep(ctx, carton); looped {
					cancel()
					once.Do(func() { loop = l })
					return
				}
			}
		}
	}

	num := runtime.NumCPU()
	wg.Add(num)
	for i := 0; i < num; i++ {
		go func() {

			detect(ctx, recv)
			wg.Done()
		}()
	}
	wg.Wait()
	return len(loop) > 0, loop
}

func hasLoopDep(ctx context.Context, carton string) (bool, []string) {
	d := new(dfs)
	d.colors = make(map[string]int)
	d.path = []string{}
	return d.cyclicUtil(ctx, carton)
}

type dfs struct {
	path   []string
	colors map[string]int
}

func (d *dfs) colorGet(vertex string) int {
	if color, ok := d.colors[vertex]; ok {
		return color
	}
	return white
}

func (d *dfs) colorSet(vertex string, color int) {
	d.colors[vertex] = color
}

func (d *dfs) cyclicUtil(ctx context.Context, vertex string) (bool, []string) {

	d.colorSet(vertex, gray)
	d.path = append(d.path, vertex)

	edges := adjacentEdges(vertex)
	for _, v := range edges {

		select {
		case <-ctx.Done():
			return false, nil
		default:
			switch d.colorGet(v) {
			case white:
				if looped, loop := d.cyclicUtil(ctx, v); looped {
					return true, loop
				}
			case gray:
				for k := range d.path {
					if d.path[k] == vertex {
						d.path = d.path[0 : k+1]
						break
					}
				}
				// fmt.Println("loop detectedx:", d.path)
				return true, d.path
			default: //continue
			}
		}
	}
	d.colorSet(vertex, black)
	d.path = d.path[0 : len(d.path)-1]
	return false, nil
}

func adjacentEdges(name string) []string {

	b, _, e := Find(name)
	if e != nil {
		log.Fatalf("carton %s: %s\n", name, e)
	}

	dep := b.BuildDepends()
	required := b.Depends()

	edges := []string{}
	edges = append(edges, dep...)
	edges = append(edges, required...)

	return edges
}

package carton

import (
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
var colors = make(map[string]int)

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
	colors[name] = white

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
// if not found, return nil as Builder
func Find(name string) (h Builder, isVirtual bool) {

	// TODO: handle if exist in both inventory
	if carton, ok := inventory[name]; ok {
		return carton, false
	}

	if virtual, ok := virtualInventory[name]; ok {
		return virtual, true
	}
	return nil, true
}

// BuildInventory build carton warehouse and then check whether each carton has
// loop dependcy hierarchy. if loop is found, it's returned in slice
func BuildInventory() (bool, []string) {
	buildInventory()
	return detectLoopDep()
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
func detectLoopDep() (bool, []string) {

	// TODO: use pipeline to check loop
	for carton := range inventory {
		if looped, loop := hasLoopDep(carton); looped {
			return looped, loop
		}
	}
	return false, nil
}

func hasLoopDep(carton string) (bool, []string) {
	path := []string{}
	if colors[carton] == white {
		return dfs(carton, path)
	}
	return false, nil
}

func adjacentEdges(name string) []string {

	b, _ := Find(name)
	if b == nil {
		log.Fatalf("carton %s: %s\n", name, ErrNotFound)
	}

	dep := b.BuildDepends()
	required := b.Depends()

	edges := []string{}
	edges = append(edges, dep...)
	edges = append(edges, required...)

	return edges
}

func dfs(vertex string, path []string) (bool, []string) {

	colors[vertex] = gray
	path = append(path, vertex)

	edges := adjacentEdges(vertex)
	for _, v := range edges {
		switch colors[v] {
		case white:
			if looped, loop := dfs(v, path); looped {
				return true, loop
			}
		case gray:
			for k := range path {
				if path[k] == vertex {
					path = path[0 : k+1]
					break
				}
			}
			// fmt.Println("loop detected:", path)
			return true, path
		default: //continue
		}
	}
	colors[vertex] = black
	path = path[0 : len(path)-1]
	return false, nil
}

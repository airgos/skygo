// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package carton

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"merge/log"
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
		panic(fmt.Sprintln("Carton Err:", ErrNoName))
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

	carton, ok := inventory[name]
	if !ok {
		log.Warning("carton %s is not found for updating", name)
		return
	}
	if m != nil {

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
// if name have suffix "-native", isNative is true and trim it before
// finding in database
// if not found, return ErrNotFound
func Find(name string) (h Builder, isVirtual bool, isNative bool, err error) {

	if strings.HasSuffix(name, "-native") {
		isNative = true
		name = strings.TrimSuffix(name, "-native")
	}

	// TODO: handle if exist in both inventory
	if carton, ok := inventory[name]; ok {
		return carton, false, isNative, nil
	}

	if virtual, ok := virtualInventory[name]; ok {
		return virtual, true, isNative, nil
	}
	return nil, true, isNative, ErrNotFound
}

// BuildInventory build carton warehouse and then check whether each carton has
// loop dependcy hierarchy.
func BuildInventory(ctx context.Context) error {
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
// has loop.
func detectLoopDep(ctx context.Context) error {

	var (
		wg   sync.WaitGroup
		once sync.Once
		err  error
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
				if e := hasLoopDep(ctx, carton); e != nil {
					cancel()
					once.Do(func() { err = e })
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
	return err
}

func hasLoopDep(ctx context.Context, carton string) error {
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

func (d *dfs) cyclicUtil(ctx context.Context, vertex string) error {

	d.colorSet(vertex, gray)
	d.path = append(d.path, vertex)

	edges, e := adjacentEdges(vertex)
	if e != nil {
		return e
	}
	for _, v := range edges {

		select {
		case <-ctx.Done():
			return nil
		default:
			switch d.colorGet(v) {
			case white:
				if e := d.cyclicUtil(ctx, v); e != nil {
					return e
				}
			case gray:
				for k := range d.path {
					if d.path[k] == vertex {
						d.path = d.path[0 : k+1]
						break
					}
				}
				return fmt.Errorf("detected loop: %v", d.path)
			default: //continue
			}
		}
	}
	d.colorSet(vertex, black)
	d.path = d.path[0 : len(d.path)-1]
	return nil
}

func adjacentEdges(name string) ([]string, error) {

	b, _, _, e := Find(name)
	if e != nil {
		return nil, fmt.Errorf("carton %s: %s", name, e)
	}

	dep := b.BuildDepends()
	required := b.Depends()

	edges := []string{}
	edges = append(edges, dep...)
	edges = append(edges, required...)

	return edges, nil
}

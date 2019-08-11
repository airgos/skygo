package carton

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
)

// hold all carton and its variants
var inventory = make(map[string]Builder)
var virtualInventory = make(map[string]Builder)

var initCh = make(chan func())
var updateCh = make(chan func())

var updateNum int

// Add add carton handler to inventory
// Generally, it's invoked by init function in carton description file
func Add(carton Builder, f func()) {

	name := carton.Provider()
	if name == "" {
		panic(fmt.Sprintf("Carton Err:", ErrNoName))
	}

	pc, file, _, _ := runtime.Caller(1)
	details := runtime.FuncForPC(pc)
	if !strings.Contains(details.Name(), ".init.") {

		panic(fmt.Errorf("%s: must add carton in init func", file))
	}

	if _, ok := inventory[name]; ok {
		panic(fmt.Sprintf("Carton Err: %s had been added!", name))
	}
	inventory[name] = carton

	// run in goroutine to improve user experience
	go func() {
		initCh <- func() {
			if init, ok := carton.(Initer); ok {
				init.Init()
				init.InstallRunbook()
			}
			carton.From(file)
			if f != nil {
				f()
			}
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

// BuildInventory build carton warehouse
func BuildInventory() {

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

// Copyright © 2020 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package load

import (
	"sync"
	"sync/atomic"

	"skygo/utils/log"
)

type states struct {
	runbooks [2]sync.Map
}

type state struct {
	ctx    *_context
	stages sync.Map
}

func index(isNative bool) int {

	index := 0
	if isNative {
		index = 1
	}
	return index
}

// loadOrStoreRunbook returns the existing state for the runbook @name if present.
// Otherwise, it stores and returns the given state.
// The loaded result is true if the value was loaded, false if stored.
func (this *states) loadOrStoreRunbook(name string, isNative bool) (*state, bool) {

	s, ok := this.runbooks[index(isNative)].LoadOrStore(name, new(state))
	return s.(*state), ok
}

// check where the stage @name owned by runbook is loaded
func (this *states) isStageLoaded(runbook, stage string, isNative bool) bool {

	if s, ok := this.runbooks[index(isNative)].Load(runbook); ok {

		state := s.(*state)

		if status, ok := state.stages.Load(stage); ok {
			if atomic.LoadInt32(status.(*int32)) == 1 {
				log.Trace("Stage %s had been stored into %s's state", stage, runbook)
				return true
			}
		}
	}
	return false
}

// storeStage mark stage in the runbook had been played
func (this *states) storeStage(runbook, stage string, isNative bool) {

	if s, ok := this.runbooks[index(isNative)].Load(runbook); ok {

		state := s.(*state)

		if status, ok := state.stages.Load(stage); ok {
			atomic.StoreInt32(status.(*int32), 1)
			log.Trace("Store %s's stage %s into state", runbook, stage)
		}
	}
}

// setCtx initialize state
func (this *state) setCtx(ctx *_context) {

	this.ctx = ctx
	rb := ctx.carton.Runbook()
	for stage := rb.Head(); stage != nil; stage = stage.Next() {
		status := new(int32)
		atomic.StoreInt32(status, 0)
		this.stages.LoadOrStore(stage.Name(), status)
	}
}

// returuns local _context
func (this *state) getCtx() *_context {

	return this.ctx
}

// Copyright © 2020 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package load

import (
	"io"
	"sync"
	"sync/atomic"

	"skygo/utils/log"
)

type states struct {
	runbooks [2]sync.Map
}

type state struct {
	ctx    *_context
	stages sync.Map // store metaStage
}

type metaStage struct {
	done           *int32
	stdout, stderr io.WriteCloser
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

// returns true if runbook is loaded in either native or cross type
func (this *states) isRunbookLoaded(runbook string) bool {

	_, native := this.runbooks[index(true)].Load(runbook)
	_, cross := this.runbooks[index(false)].Load(runbook)
	return native || cross
}

// check where the stage @name owned by runbook is loaded
func (this *states) isStageLoaded(runbook, stage string, isNative bool) bool {

	if meta := this.getStage(runbook, stage, isNative); meta != nil {
		if atomic.LoadInt32(meta.done) == 1 {
			log.Trace("Stage %s had been cached into %s's state", stage, runbook)
			return true
		}
	}
	return false
}

// storeStage mark stage in the runbook had been played
func (this *states) setStageDone(runbook, stage string, isNative bool) {

	if meta := this.getStage(runbook, stage, isNative); meta != nil {
		atomic.StoreInt32(meta.done, 1)
		log.Trace("Cache %s's stage %s into state", runbook, stage)
	}
}

func (this *states) getStage(runbook, stage string, isNative bool) *metaStage {

	if s, ok := this.runbooks[index(isNative)].Load(runbook); ok {

		state := s.(*state)
		if meta, ok := state.stages.Load(stage); ok {
			return meta.(*metaStage)
		}
	}
	return nil
}

func (meta *metaStage) setIO(stdout, stderr io.WriteCloser) {
	meta.stdout, meta.stderr = stdout, stderr
}

func (meta *metaStage) getIO() (stdout, stderr io.WriteCloser) {
	return meta.stdout, meta.stderr
}

// setCtx initialize state
func (this *state) setCtx(ctx *_context) {

	this.ctx = ctx
	rb := ctx.carton.Runbook()
	for stage := rb.Head(); stage != nil; stage = stage.Next() {
		meta := new(metaStage)
		meta.done = new(int32)
		atomic.StoreInt32(meta.done, 0)
		this.stages.LoadOrStore(stage.Name(), meta)
	}
}

// returuns local _context
func (this *state) getCtx() *_context {

	return this.ctx
}

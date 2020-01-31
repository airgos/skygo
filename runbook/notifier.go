// Copyright © 2020 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runbook

import (
	"container/list"
	"sync"
)

// Notifer define notifier prototype
type Notifer func(Context, string) error

type NotifierKind int

const (
	ENTER NotifierKind = iota // entrance notifier chain
	EXIT                      // exit notifier chain
	RESET                     // reset notifier chain
	END
)

type notifierChain struct {
	notifiers [END]struct {
		chain list.List
		m     sync.Mutex
	}
}

// init initialize notifier chain
func (chain *notifierChain) init() {

	// init notifier chain
	for i := 0; i < int(END); i++ {
		chain.notifiers[i].chain.Init()
	}
}

// RegisterNotifier register notifier function to notifier chain
// when notifier chain is iterated, it depends on kind
func (chain *notifierChain) RegisterNotifier(n Notifer, kind NotifierKind) {

	chain.notifiers[kind].m.Lock()
	defer chain.notifiers[kind].m.Unlock()

	if kind < END {
		chain.notifiers[kind].chain.PushFront(n)
	}
}

// registerNotifier register notifier function to the end of notifier chain
// when notifier chain is iterated, it depends on kind
func (chain *notifierChain) registerNotifierBack(n Notifer, kind NotifierKind) {

	chain.notifiers[kind].m.Lock()
	defer chain.notifiers[kind].m.Unlock()

	if kind < END {
		chain.notifiers[kind].chain.PushBack(n)
	}
}

// callNotifierChain iterates notifier chain one by one
// abort if error occurs on any notifier
func (chain *notifierChain) callNotifierChain(ctx Context, kind NotifierKind, name string) error {

	chain.notifiers[kind].m.Lock()
	defer chain.notifiers[kind].m.Unlock()

	notifiers := chain.notifiers[kind].chain

	// delete node during iterating
	var next *list.Element
	for e := notifiers.Front(); e != nil; e = next {
		n := e.Value.(Notifer)
		if err := n(ctx, name); err != nil {
			return err
		}

		next = e.Next()
		notifiers.Remove(e)
	}
	return nil
}

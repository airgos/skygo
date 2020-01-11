// Copyright © 2020 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package load

import (
	"sync/atomic"
)

type refcount struct {
	count   int32
	release func()
}

func (ref *refcount) refGet() {
	atomic.AddInt32(&ref.count, 1)
}

func (ref *refcount) refPut(release func()) {
	atomic.AddInt32(&ref.count, -1)
	if 0 == atomic.LoadInt32(&ref.count) {
		release()
	}
}

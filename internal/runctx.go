// Copyright 2022 Jeremy Edwards
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal

import (
	"os"
	"sync"
)

// RunCtx is a context for running a program that can be killed and waited on. It is used to manage the lifecycle of a program and its goroutines.
type RunCtx struct {
	sync.RWMutex
	waitCh chan os.Signal
	closed bool
}

// NewRunCtx creates a new [RunCtx].
func NewRunCtx() *RunCtx {
	return &RunCtx{
		waitCh: make(chan os.Signal, 1),
		closed: false,
	}
}

// Kill sends a kill signal to the [RunCtx]. If the [RunCtx] is already closed, it does nothing. If there is already a pending kill signal, it does nothing.
func (mc *RunCtx) Kill() {
	mc.RLock()
	defer mc.RUnlock()
	if mc.closed {
		return
	}
	select {
	case mc.waitCh <- os.Kill:
	default:
		// waitCh already has a pending kill signal buffered; nothing more to do.
	}
}

// Wait blocks until the [RunCtx] is closed. It returns immediately if the [RunCtx] is already closed.
func (mc *RunCtx) Wait() {
	<-mc.waitCh
}

// Close closes the [RunCtx]. It is safe to call multiple times. If the [RunCtx] is already closed, it does nothing.
func (mc *RunCtx) Close() {
	doClose := false
	mc.Lock()
	if !mc.closed {
		doClose = true
		mc.closed = true
	}

	mc.Unlock()
	if doClose {
		close(mc.waitCh)
	}
}

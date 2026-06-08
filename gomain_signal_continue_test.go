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

//go:build !windows && !plan9 && !js
// +build !windows,!plan9,!js

package gomain

import (
	"os"
	"syscall"
	"testing"
	"time"
)

// A signal that handleSignal reports as non-terminal (returns false, e.g.
// SIGUSR1 which only dumps a stack trace per the README) must not cause
// runInteractiveInternal to tear down the run context. The MainFunc should
// keep running, blocked on wait(), until a terminal signal arrives.
func TestRunInteractiveInternalKeepsRunningAfterNonTerminalSignal(t *testing.T) {
	sigCh := make(chan os.Signal, 1)

	mainDone := make(chan struct{})
	mainFunc := func(wait func()) error {
		wait()
		close(mainDone)
		return nil
	}

	internalDone := make(chan struct{})
	go func() {
		runInteractiveInternal(mainFunc, sigCh)
		close(internalDone)
	}()

	sigCh <- syscall.SIGUSR1

	select {
	case <-mainDone:
		t.Fatal("MainFunc's wait() returned after a non-terminal signal (SIGUSR1); run context was torn down prematurely")
	case <-internalDone:
		t.Fatal("runInteractiveInternal returned after a non-terminal signal (SIGUSR1) instead of continuing to run")
	case <-time.After(300 * time.Millisecond):
		// Expected: still running.
	}

	sigCh <- syscall.SIGTERM

	select {
	case <-internalDone:
	case <-time.After(2 * time.Second):
		t.Fatal("runInteractiveInternal did not return after a terminal signal (SIGTERM)")
	}
	<-mainDone
}

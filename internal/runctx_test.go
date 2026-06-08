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
	"testing"
	"time"
)

func TestNewRunCtx(t *testing.T) {
	rCtx := NewRunCtx()

	rCtx.Close()
	rCtx.Kill()
	rCtx.Close()
	rCtx.Kill()
	rCtx.Close()
	rCtx.Close()
	rCtx.Kill()
	rCtx.Kill()
}

func TestWaitAfterKill(t *testing.T) {
	rCtx := NewRunCtx()

	rCtx.Kill()
	rCtx.Close()
	rCtx.Wait()
}

func TestKillTwiceDoesNotDeadlock(t *testing.T) {
	rCtx := NewRunCtx()

	killDone := make(chan struct{})
	go func() {
		rCtx.Kill()
		rCtx.Kill() // second call: waitCh's buffer is already full
		close(killDone)
	}()

	select {
	case <-killDone:
	case <-time.After(2 * time.Second):
		t.Fatal("calling Kill() twice deadlocked: the second call blocked sending " +
			"on the full waitCh buffer while still holding RunCtx's lock")
	}

	closeDone := make(chan struct{})
	go func() {
		rCtx.Close()
		close(closeDone)
	}()

	select {
	case <-closeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Close() deadlocked: it could not acquire the lock held by the blocked Kill() call")
	}
}

func TestWait(t *testing.T) {
	rCtx := NewRunCtx()
	done := make(chan bool, 1)

	go func() {
		rCtx.Wait()
		done <- true
	}()

	time.Sleep(time.Millisecond * 100)
	rCtx.Kill()
	<-done
}

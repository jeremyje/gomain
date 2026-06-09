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

package gomain

import (
	"fmt"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

var (
	waitForeverFuncs = map[string]MainFunc{
		"waitForeverFunc": func(wait func()) error {
			wait()
			return nil
		},
		"waitForeverFailingFunc": func(wait func()) error {
			wait()
			return fmt.Errorf("failed")
		},
	}

	immediateMainFuncs = map[string]MainFunc{
		"immediateReturnFunc": func(wait func()) error {
			return nil
		},
		"immediateFailFunc": func(wait func()) error {
			return fmt.Errorf("failed")
		},
	}
)

func getAllMainFuncs() map[string]MainFunc {
	mains := map[string]MainFunc{}
	for k, v := range waitForeverFuncs {
		mains[k] = v
	}
	for k, v := range immediateMainFuncs {
		mains[k] = v
	}
	return mains
}

func TestGetTerminalSignalsBaseDoesNotRegisterUncatchableSignals(t *testing.T) {
	for _, sig := range getTerminalSignalsBase() {
		if sig == syscall.SIGKILL {
			t.Errorf("getTerminalSignalsBase() registers %s, but SIGKILL can never be delivered "+
				"to a process's signal handler (POSIX prohibits catching, blocking, or ignoring it), "+
				"so signal.Notify(..., SIGKILL) is a silent no-op", sig)
		}
	}
}

func TestHandleSignalDumpsStackAndStopsOnSIGQUIT(t *testing.T) {
	// SIGQUIT is the conventional catchable "dump a stack trace, then stop"
	// signal (unlike SIGKILL, which can never reach a handler).
	if got := handleSignal(syscall.SIGQUIT, Config{}); !got {
		t.Errorf("handleSignalBase(SIGQUIT) = %t, want true (terminal signal that dumps a stack trace)", got)
	}
}

func TestHandleSignal(t *testing.T) {
	for _, tc := range handleSignalTestCases {
		tc := tc
		t.Run(tc.input.String(), func(t *testing.T) {
			t.Parallel()
			got := handleSignal(tc.input, Config{})
			if got != tc.want {
				t.Fatalf("expected: %t, got: %t", tc.want, got)
			}
		})
	}
}

func TestRunInteractiveInternal(t *testing.T) {
	if testing.Short() {
		t.Skip("time.Sleep usage")
	}
	for mainName, mainFunc := range getAllMainFuncs() {
		mainFunc := mainFunc
		for _, tc := range handleSignalTestCases {
			tc := tc

			t.Run(fmt.Sprintf("%s - %s", mainName, tc.input.String()), func(t *testing.T) {
				t.Parallel()
				sigCh := make(chan os.Signal, 1)

				go func() {
					time.Sleep(time.Millisecond * 100)
					sigCh <- tc.input
					if !tc.want {
						// tc.input is non-terminal (e.g. SIGUSR1, which only
						// dumps a stack trace): runInteractiveInternal must
						// keep running, so send a terminal signal too or this
						// test would hang forever.
						time.Sleep(time.Millisecond * 100)
						sigCh <- syscall.SIGTERM
					}
				}()

				runInteractiveInternal(mainFunc, sigCh, Config{})
			})
		}
	}
}

func TestRunInteractiveInternalAllSignals(t *testing.T) {
	if testing.Short() {
		t.Skip("time.Sleep usage")
	}
	for mainName, mainFunc := range getAllMainFuncs() {
		mainFunc := mainFunc
		for _, signal := range getAllSignals() {
			signal := signal

			t.Run(fmt.Sprintf("%s - %s", mainName, signal.String()), func(t *testing.T) {
				t.Parallel()
				sigCh := make(chan os.Signal, 1)
				var m sync.Mutex
				closed := false
				defer func() {
					m.Lock()
					closed = true
					m.Unlock()
					close(sigCh)
				}()

				go func() {
					time.Sleep(time.Millisecond * 100)
					m.Lock()
					if !closed {
						sigCh <- signal
					}
					m.Unlock()

					// Most signals in getAllSignals() are non-terminal (e.g.
					// SIGCHLD, SIGALRM): runInteractiveInternal must keep
					// running after them. Follow up with a terminal signal
					// (best-effort, non-blocking) so the test can complete;
					// it's a no-op if runInteractiveInternal already returned.
					time.Sleep(time.Millisecond * 100)
					m.Lock()
					if !closed {
						select {
						case sigCh <- syscall.SIGTERM:
						default:
						}
					}
					m.Unlock()
				}()

				runInteractiveInternal(mainFunc, sigCh, Config{})
			})
		}
	}
}

func TestRunInteractiveAllSignals(t *testing.T) {
	for mainName, mainFunc := range immediateMainFuncs {
		mainFunc := mainFunc
		for _, signal := range getAllSignals() {
			signal := signal
			t.Run(fmt.Sprintf("%s - %s", mainName, signal), func(t *testing.T) {
				t.Parallel()
				runInteractive(mainFunc, Config{})
			})

			t.Run(fmt.Sprintf("%s - %s", mainName, signal), func(t *testing.T) {
				t.Parallel()
				Run(mainFunc, Config{})
			})
		}
	}
}

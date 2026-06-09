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
)

var (
	handleSignalTestCases = []struct {
		input os.Signal
		want  bool
	}{
		{input: syscall.SIGUSR1, want: false},
		{input: syscall.SIGINT, want: true},
		{input: syscall.SIGTERM, want: true},
		// SIGKILL can never be delivered to a signal handler (POSIX
		// prohibits catching, blocking, or ignoring it); it must not be
		// treated as a terminal signal here.
		{input: syscall.SIGKILL, want: false},
		{input: syscall.SIGQUIT, want: true},
		{input: syscall.SIGABRT, want: true},
	}
)

func getAllSignals() []os.Signal {
	return []os.Signal{
		syscall.SIGABRT,
		syscall.SIGALRM,
		syscall.SIGBUS,
		syscall.SIGCHLD,
		syscall.SIGCLD,
		syscall.SIGCONT,
		syscall.SIGFPE,
		syscall.SIGHUP,
		syscall.SIGILL,
		syscall.SIGINT,
		syscall.SIGIO,
		syscall.SIGIOT,
		syscall.SIGKILL,
		syscall.SIGPIPE,
		syscall.SIGPOLL,
		syscall.SIGPROF,
		syscall.SIGPWR,
		syscall.SIGQUIT,
		syscall.SIGSEGV,
		syscall.SIGSTKFLT,
		syscall.SIGSTOP,
		syscall.SIGSYS,
		syscall.SIGTERM,
		syscall.SIGTRAP,
		syscall.SIGTSTP,
		syscall.SIGTTIN,
		syscall.SIGTTOU,
		syscall.SIGUNUSED,
		syscall.SIGURG,
		syscall.SIGUSR1,
		syscall.SIGUSR2,
		syscall.SIGVTALRM,
		syscall.SIGWINCH,
		syscall.SIGXCPU,
		syscall.SIGXFSZ,
	}
}

func TestHandleSignalSIGUSR1Gating(t *testing.T) {
	testCases := []struct {
		name string
		cfg  Config
	}{
		{name: "debug disabled", cfg: Config{}},
		{name: "debug enabled", cfg: Config{Debug: true}},
		{name: "debug and sensitive enabled", cfg: Config{Debug: true, DebugSensitive: true}},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := handleSignal(syscall.SIGUSR1, tc.cfg)
			if got != false {
				t.Fatalf("expected SIGUSR1 to leave the process running (return false), got: %t", got)
			}
		})
	}
}

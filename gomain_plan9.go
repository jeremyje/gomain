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

//go:build plan9
// +build plan9

package gomain

import (
	"os"
	"syscall"
)

func platformRun(f MainFunc, cfg Config) {
	runInteractive(f)
}

func getTerminalSignals() []os.Signal {
	return append(getTerminalSignalsBase(), syscall.SIGTERM, syscall.SIGABRT)
}

func handleSignal(sig os.Signal) bool {
	switch sig {
	case syscall.SIGTERM:
		return true
	case syscall.SIGABRT:
		logStackDump()
		return true
	default:
		return handleSignalBase(sig)
	}
}

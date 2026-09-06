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
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cloudfra/gomain/internal"
)

type MainCtx interface {
	Wait()
}

type MainFunc func(func()) error

type Config struct {
	ServiceName        string
	ServiceDescription string
	Command            string
	Debug              bool
	DebugSensitive     bool
}

func Run(f MainFunc, cfg Config) {
	platformRun(f, cfg)
}

func runInteractive(f MainFunc, cfg Config) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, getTerminalSignals()...)
	defer func() {
		signal.Stop(sigCh)
		close(sigCh)
	}()
	runInteractiveInternal(f, sigCh, cfg)
}

func runInteractiveInternal(f MainFunc, sigCh chan os.Signal, cfg Config) {
	mainErrCh := make(chan error, 1)

	mc := internal.NewRunCtx()
	defer mc.Close()

	go func() {
		mainErrCh <- f(mc.Wait)
		close(mainErrCh)
	}()

	for {
		select {
		case err := <-mainErrCh:
			handleError(err)
			return
		case sig := <-sigCh:
			if handleSignal(sig, cfg) {
				signal.Stop(sigCh)
				mc.Kill()
				return
			}
		}
	}
}

func handleError(err error) {
	if err != nil {
		log.Printf("ERROR: %s", err)
	}
}

func getTerminalSignalsBase() []os.Signal {
	return []os.Signal{syscall.SIGINT}
}

func handleSignalBase(sig os.Signal) bool {
	switch sig {
	case syscall.SIGINT:
		return true
	default:
		return false
	}
}

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
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/cloudfra/gomain/internal"
)

// MainCtx is the interface for a context that can be waited on. It is used to manage the lifecycle of a program and its goroutines.
type MainCtx interface {
	Wait()
}

// MainFunc is the type of the main function that is run by [Run]. It takes a function that can be called to wait for a kill signal. It returns an error if the program should exit with an error.
type MainFunc func(func()) error

// Config is the configuration for running a gomain program.
type Config struct {
	ServiceName        string
	ServiceDescription string
	Command            string
	Debug              bool
	DebugSensitive     bool
}

// Run runs the given [MainFunc] with the given [Config]. It handles signals and logging. It is the main entry point for a gomain program.
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
		slog.With(err).Error("application error")
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

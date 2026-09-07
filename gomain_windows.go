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

//go:build windows
// +build windows

package gomain

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/cloudfra/gomain/internal"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	errorCodeBadArguments        = 2
	errorCodeNotRunningAsService = 3
)

func platformRun(f MainFunc, cfg Config) {
	svcMode, err := svc.IsWindowsService()
	if err != nil {
		slog.With(err).Error("failed to determine if we are running in service")
		os.Exit(errorCodeNotRunningAsService)
	}
	if svcMode {
		runService(f, cfg.ServiceName, false)
	} else {
		if cfg.Command != "" {
			serviceControl(f, cfg)
		} else {
			runInteractive(f, cfg)
		}
	}
}

func serviceControl(f MainFunc, cfg Config) {
	var err error
	svcName := cfg.ServiceName
	description := cfg.ServiceDescription
	cmd := cfg.Command

	switch strings.ToLower(cmd) {
	case "debug":
		runService(f, svcName, true)
		return
	case "install":
		err = installService(svcName, description)
	case "remove":
		err = removeService(svcName)
	case "start":
		err = startService(svcName)
	case "stop":
		err = controlService(svcName, svc.Stop, svc.Stopped)
	case "pause":
		err = controlService(svcName, svc.Pause, svc.Paused)
	case "continue":
		err = controlService(svcName, svc.Continue, svc.Running)
	default:
		usage(fmt.Sprintf("invalid command %s", cmd))
	}
	handleError(err)
}

func usage(errmsg string) {
	slog.Warn(
		"%s\n\n"+
			"usage: %s <command>\n"+
			"       where <command> is one of\n"+
			"       install, remove, debug, start, stop, pause or continue.\n",
		errmsg, os.Args[0])
	os.Exit(errorCodeBadArguments)
}

func installService(name, desc string) error {
	exepath := exePath()
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func() {
		if err := m.Disconnect(); err != nil {
			slog.With(err).Warn("failed to disconnect to service manager")
		}
	}()

	s, err := m.OpenService(name)
	if err == nil {
		closeErr := s.Close()
		return joinErrors(fmt.Errorf("service %s already exists", name), closeErr)
	}

	s, err = m.CreateService(name, exepath, mgr.Config{DisplayName: desc}, "is", "auto-started")
	if err != nil {
		return err
	}

	defer func() {
		if err := s.Close(); err != nil {
			slog.With(err).Warn("failed to close service manager connection")
		}
	}()
	err = eventlog.InstallAsEventCreate(name, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		deleteErr := s.Delete()
		return joinErrors(fmt.Errorf("SetupEventLogSource() failed: %s", err), deleteErr)
	}
	return nil
}

func removeService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func() {
		if err := m.Disconnect(); err != nil {
			slog.With(err).Warn("failed to disconnect from service manager")
		}
	}()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("service %s is not installed", name)
	}
	defer func() {
		if err := s.Close(); err != nil {
			slog.With(err).Warn("failed to close service manager connection")
		}
	}()
	err = s.Delete()
	if err != nil {
		return err
	}
	err = eventlog.Remove(name)
	if err != nil {
		return fmt.Errorf("RemoveEventLogSource() failed: %s", err)
	}
	return nil
}

func startService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func() {
		if err := m.Disconnect(); err != nil {
			slog.With(err).Warn("failed to disconnect from service manager")
		}
	}()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			slog.With(err).Warn("failed to close service manager connection")
		}
	}()
	err = s.Start("is", "manual-started")
	if err != nil {
		return fmt.Errorf("could not start service: %v", err)
	}
	return nil
}

func controlService(name string, c svc.Cmd, to svc.State) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func() {
		if err := m.Disconnect(); err != nil {
			slog.With(err).Warn("failed to disconnect from service manager")
		}
	}()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			slog.With(err).Warn("failed to close service manager connection")
		}
	}()
	status, err := s.Control(c)
	if err != nil {
		return fmt.Errorf("could not send control=%d: %v", c, err)
	}
	timeout := time.Now().Add(10 * time.Second)
	for status.State != to {
		if timeout.Before(time.Now()) {
			return fmt.Errorf("timeout waiting for service to go to state=%d", to)
		}
		time.Sleep(300 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			return fmt.Errorf("could not retrieve service status: %v", err)
		}
	}
	return nil
}

var elog debug.Log

type windowsService struct {
	f MainFunc
}

func (ws *windowsService) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	mc := internal.NewRunCtx()
	defer mc.Close()

	mainErrCh := make(chan error, 1)
	go func() {
		mainErrCh <- ws.f(mc.Wait)
	}()
	defer close(mainErrCh)

	done := false
	for !done {
		select {
		case cr := <-r:
			if ws.handleControl(cr, changes) {
				mc.Kill()
			}
		case err := <-mainErrCh:
			if err != nil {
				reportEventLogError(elog.Error(1, err.Error()))
			}
			done = true
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	changes <- svc.Status{State: svc.Stopped}
	return
}

func (ws *windowsService) handleControl(cr svc.ChangeRequest, changes chan<- svc.Status) bool {
	done := false
	switch cr.Cmd {
	case svc.Interrogate:
		changes <- cr.CurrentStatus
		// Testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
		time.Sleep(100 * time.Millisecond)
		changes <- cr.CurrentStatus
	case svc.Stop, svc.Shutdown:
		reportEventLogError(elog.Info(1, "Stopping Service"))
		done = true
	default:
		reportEventLogError(elog.Error(1, fmt.Sprintf("unexpected control request #%d", cr)))
	}
	return done
}

func runService(f MainFunc, name string, isDebug bool) {
	var err error
	if isDebug {
		elog = debug.New(name)
	} else {
		elog, err = eventlog.Open(name)
		if err != nil {
			reportEventLogError(err)
			return
		}
	}
	defer func() {
		if err := elog.Close(); err != nil {
			slog.With(err).Warn("failed to close event log connection")
		}
	}()

	reportEventLogError(elog.Info(1, fmt.Sprintf("starting %s service", name)))
	run := svc.Run
	if isDebug {
		run = debug.Run
	}
	err = run(name, &windowsService{
		f: f,
	})
	if err != nil {
		reportEventLogError(elog.Error(1, fmt.Sprintf("%s service failed: %v", name, err)))
		return
	}
	reportEventLogError(elog.Info(1, fmt.Sprintf("%s service stopped", name)))
}

func getTerminalSignals() []os.Signal {
	return append(getTerminalSignalsBase(), syscall.SIGTERM, syscall.SIGABRT, syscall.SIGQUIT)
}

func handleSignal(sig os.Signal, _ Config) bool {
	switch sig {
	case syscall.SIGTERM:
		return true
	case syscall.SIGABRT, syscall.SIGQUIT:
		logStackDump()
		return true
	default:
		return handleSignalBase(sig)
	}
}

func reportEventLogError(err error) {
	if err != nil {
		slog.With(err).Warn("failed to write log entry to event log")
	}
}

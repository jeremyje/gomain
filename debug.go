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

// Package gomain provides a framework for building Go applications that can handle graceful shutdowns and signal handling. It allows developers to define a main function that can be run in a controlled context, enabling better management of application lifecycle events such as termination signals and cleanup operations.
package gomain

import (
	"fmt"
	"log/slog"
	"math"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

var processStartTime = time.Now()

func logStackDump() {
	slog.Debug(string(getStackDump()))
}

func getRuntimeInfo() string {
	return fmt.Sprintf(
		"Go Version: %s\nGOOS/GOARCH: %s/%s\nNumCPU: %d\nGOMAXPROCS: %d\nNumGoroutine: %d\n",
		runtime.Version(), runtime.GOOS, runtime.GOARCH,
		runtime.NumCPU(), runtime.GOMAXPROCS(0), runtime.NumGoroutine())
}

func getProcessInfo() string {
	wd, err := os.Getwd()
	if err != nil {
		wd = fmt.Sprintf("unknown (%s)", err)
	}
	hostname, err := os.Hostname()
	if err != nil {
		hostname = fmt.Sprintf("unknown (%s)", err)
	}
	return fmt.Sprintf(
		"PID: %d\nExecutable: %s\nWorking Directory: %s\nHostname: %s\nStart Time: %s\nUptime: %s\n",
		os.Getpid(), exePath(), wd, hostname,
		processStartTime.Format(time.RFC3339), time.Since(processStartTime))
}

func getMemoryStats() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	var pause time.Duration
	if m.PauseTotalNs > uint64(math.MaxInt64) {
		pause = time.Duration(math.MaxInt64)
	} else {
		pause = time.Duration(int64(m.PauseTotalNs))
	}

	var lastGC time.Time
	if m.LastGC == 0 {
		lastGC = time.Time{}
	} else {
		if m.LastGC > uint64(math.MaxInt64) {
			lastGC = time.Unix(0, int64(math.MaxInt64))
		} else {
			lastGC = time.Unix(0, int64(m.LastGC))
		}
	}

	return fmt.Sprintf(
		"HeapAlloc: %d bytes\nHeapSys: %d bytes\nHeapObjects: %d\nStackInuse: %d bytes\n"+
			"NumGC: %d\nTotalGCPause: %s\nLastGC: %s\nGCCPUFraction: %f\n",
		m.HeapAlloc, m.HeapSys, m.HeapObjects, m.StackInuse,
		m.NumGC, pause, lastGC, m.GCCPUFraction)
}

func getBuildInfo() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "Build Info: unavailable\n"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Main Module: %s %s\n", info.Main.Path, info.Main.Version)
	fmt.Fprintf(&sb, "Go Version: %s\n", info.GoVersion)
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision", "vcs.time", "vcs.modified":
			fmt.Fprintf(&sb, "%s: %s\n", setting.Key, setting.Value)
		}
	}
	if len(info.Deps) > 0 {
		sb.WriteString("Dependencies:\n")
		for _, dep := range info.Deps {
			fmt.Fprintf(&sb, "  %s %s\n", dep.Path, dep.Version)
		}
	}
	return sb.String()
}

func getSensitiveInfo() string {
	var sb strings.Builder
	sb.WriteString("Args:\n")
	for _, arg := range os.Args {
		fmt.Fprintf(&sb, "  %s\n", arg)
	}
	sb.WriteString("Environment:\n")
	for _, env := range os.Environ() {
		fmt.Fprintf(&sb, "  %s\n", env)
	}
	return sb.String()
}

func getDebugDump(cfg Config) []byte {
	var sb strings.Builder
	sb.WriteString("=== Runtime ===\n")
	sb.WriteString(getRuntimeInfo())
	sb.WriteString("=== Build Info ===\n")
	sb.WriteString(getBuildInfo())
	sb.WriteString("=== Memory & GC ===\n")
	sb.WriteString(getMemoryStats())
	sb.WriteString("=== Process Info ===\n")
	sb.WriteString(getProcessInfo())
	if cfg.DebugSensitive {
		sb.WriteString("=== Sensitive Info ===\n")
		sb.WriteString(getSensitiveInfo())
	}
	sb.WriteString("=== Goroutine Stack Dump ===\n")
	sb.Write(getStackDump())
	return []byte(sb.String())
}

func logDebugDump(cfg Config) {
	slog.Debug(string(getDebugDump(cfg)))
}

func getStackDump() []byte {
	var (
		buf       []byte
		stackSize int
	)
	bufferLen := 16384
	for stackSize == len(buf) {
		buf = make([]byte, bufferLen)
		stackSize = runtime.Stack(buf, true)
		bufferLen *= 2
	}
	buf = buf[:stackSize]
	return buf
}

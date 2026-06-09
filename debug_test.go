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
	"runtime"
	"strings"
	"testing"
)

func TestGetStackDump(t *testing.T) {
	dump := string(getStackDump())
	if !strings.Contains(dump, t.Name()) {
		t.Errorf("expected '%s' in stack dump\n%s", t.Name(), dump)
	}

	// Make sure this doesn't crash or something weird.
	logStackDump()
}

func TestGetRuntimeInfo(t *testing.T) {
	info := getRuntimeInfo()
	for _, want := range []string{runtime.Version(), runtime.GOOS, runtime.GOARCH} {
		if !strings.Contains(info, want) {
			t.Errorf("expected runtime info to contain %q\n%s", want, info)
		}
	}
}

func TestGetBuildInfo(t *testing.T) {
	info := getBuildInfo()
	if !strings.Contains(info, "github.com/jeremyje/gomain") {
		t.Errorf("expected build info to contain the main module path\n%s", info)
	}
}

func TestGetMemoryStats(t *testing.T) {
	stats := getMemoryStats()
	for _, want := range []string{"HeapAlloc", "HeapSys", "NumGC"} {
		if !strings.Contains(stats, want) {
			t.Errorf("expected memory stats to contain %q\n%s", want, stats)
		}
	}
}

func TestGetProcessInfo(t *testing.T) {
	info := getProcessInfo()
	pid := fmt.Sprintf("%d", os.Getpid())
	for _, want := range []string{pid, exePath()} {
		if !strings.Contains(info, want) {
			t.Errorf("expected process info to contain %q\n%s", want, info)
		}
	}
}

func TestGetSensitiveInfo(t *testing.T) {
	info := getSensitiveInfo()
	for _, want := range []string{"Args:", "Environment:", os.Args[0]} {
		if !strings.Contains(info, want) {
			t.Errorf("expected sensitive info to contain %q\n%s", want, info)
		}
	}
}

func TestGetDebugDump(t *testing.T) {
	dump := string(getDebugDump(Config{}))
	for _, want := range []string{
		"=== Runtime ===",
		"=== Build Info ===",
		"=== Memory & GC ===",
		"=== Process Info ===",
		"=== Goroutine Stack Dump ===",
		runtime.Version(),
		t.Name(), // present because getStackDump's output includes the calling goroutine
	} {
		if !strings.Contains(dump, want) {
			t.Errorf("expected debug dump to contain %q\n%s", want, dump)
		}
	}
	if strings.Contains(dump, "=== Sensitive Info ===") {
		t.Errorf("expected debug dump to omit sensitive info when DebugSensitive is false\n%s", dump)
	}
}

func TestGetDebugDumpSensitive(t *testing.T) {
	dump := string(getDebugDump(Config{DebugSensitive: true}))
	for _, want := range []string{"=== Sensitive Info ===", os.Args[0]} {
		if !strings.Contains(dump, want) {
			t.Errorf("expected debug dump to contain %q when DebugSensitive is true\n%s", want, dump)
		}
	}
}

func TestLogDebugDump(t *testing.T) {
	// Make sure this doesn't crash or something weird, with and without sensitive info.
	logDebugDump(Config{})
	logDebugDump(Config{DebugSensitive: true})
}

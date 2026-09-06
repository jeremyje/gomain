# Extended SIGUSR1 Debug Dump Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the existing `SIGUSR1` handler (non-Windows only) so that, when opted into via new `Config.Debug`/`Config.DebugSensitive` fields, it logs a rich debug dump (Go runtime/build info, memory & GC stats, process info, optionally args/env, and the existing goroutine stack dump) instead of the current bare stack dump — and gates `SIGUSR1` so it does nothing unless `Debug` is enabled.

**Architecture:** Add small, independently-testable string-builder functions to `debug.go` for each info category, assemble them in a new `getDebugDump(cfg Config) []byte` / `logDebugDump(cfg Config)` pair (mirroring the existing `getStackDump`/`logStackDump` split), and thread a new `cfg Config` parameter through `runInteractive` → `runInteractiveInternal` → `handleSignal` so the non-Windows `SIGUSR1` case can branch on `cfg.Debug`/`cfg.DebugSensitive`.

**Tech Stack:** Go 1.25, standard library only (`runtime`, `runtime/debug`, `os`, `time`, `strings`, `fmt`, `log`).

**Reference spec:** `docs/superpowers/specs/2026-06-08-extended-debug-dump-design.md`

---

## Important context for the engineer

- This is the `gomain` library (`github.com/cloudfra/gomain`) — a main-loop harness for long-running Go apps. It wraps `os/signal` handling, with platform-specific files selected via Go build tags: `gomain_nonwindows.go` (`!windows && !plan9 && !js`), `gomain_windows.go`, `gomain_plan9.go`, `gomain_js.go`. Only ONE of these compiles for a given `GOOS`, but all four define the same functions (`getTerminalSignals`, `handleSignal`) called from the shared `gomain.go`, so their signatures must stay identical.
- You'll be running tests on Linux, so only `gomain_nonwindows.go`/`gomain_nonwindows_test.go` will actually compile and run in this environment. The Windows/Plan9/JS files must still be edited to keep their function signatures consistent (the Go compiler won't catch cross-platform signature mismatches for you on Linux), but you cannot run their tests here — just match the pattern carefully.
- Run tests from the project root: `go test -run <TestName> -v .` (the `.` targets the root package; omitting it runs all packages including `internal`, `testing`, `cmd/example`).
- Existing pattern to mirror: `debug.go` has `getStackDump() []byte` (builds the data) and `logStackDump()` (logs it via `log.Printf("%s", ...)`). `debug_test.go` tests `getStackDump` by asserting the output `strings.Contains` an expected substring (the test's own name, since `runtime.Stack` includes the calling goroutine's function name), then calls `logStackDump()` just to make sure it doesn't panic.
- `runtime/debug.ReadBuildInfo()` was confirmed (by running a scratch test in this exact module) to return `info.Main.Path == "github.com/cloudfra/gomain"` when run via `go test` here — that's the one substring you can reliably assert on. `info.Settings` (VCS info) and `info.Deps` may be empty depending on how the binary was built — don't assert on them being present, just include them in the output when they exist.
- `util.go` already has `exePath() string` — reuse it for the executable path; don't duplicate that logic.

---

### Task 1: Add `Debug`/`DebugSensitive` fields to `Config`

**Files:**

- Modify: `gomain.go:32-36`

- [ ] **Step 1: Add the two new fields to the `Config` struct**

In `gomain.go`, change:

```go
type Config struct {
  ServiceName  string
  ServiceDescription string
  Command      string
}
```

to:

```go
type Config struct {
  ServiceName  string
  ServiceDescription string
  Command      string
  Debug        bool
  DebugSensitive     bool
}
```

- [ ] **Step 2: Verify everything still builds**

Run: `go build ./...`
Expected: no output, exit code 0 (adding zero-value-safe fields to a struct is non-breaking).

- [ ] **Step 3: Commit**

```bash
git add gomain.go
git commit -m "Add Debug and DebugSensitive fields to Config"
```

---

### Task 2: Add runtime & build info helpers

**Files:**

- Modify: `debug.go`
- Test: `debug_test.go`
- [ ] **Step 1: Write the failing test for `getRuntimeInfo`**

Add to `debug_test.go` (keep the existing `TestGetStackDump` as-is):

```go
func TestGetRuntimeInfo(t *testing.T) {
  info := getRuntimeInfo()
  for _, want := range []string{runtime.Version(), runtime.GOOS, runtime.GOARCH} {
    if !strings.Contains(info, want) {
      t.Errorf("expected runtime info to contain %q\n%s", want, info)
    }
  }
}
```

Add `"runtime"` to the import block in `debug_test.go` (it currently imports `"strings"` and `"testing"`).

- [ ] **Step 2: Run it to verify it fails to compile (function doesn't exist yet)**

Run: `go test -run TestGetRuntimeInfo -v .`
Expected: FAIL — `undefined: getRuntimeInfo`

- [ ] **Step 3: Implement `getRuntimeInfo`**

Add to `debug.go` (and add `"fmt"` to its imports — it currently imports `"log"` and `"runtime"`):

```go
func getRuntimeInfo() string {
  return fmt.Sprintf(
    "Go Version: %s\nGOOS/GOARCH: %s/%s\nNumCPU: %d\nGOMAXPROCS: %d\nNumGoroutine: %d\n",
    runtime.Version(), runtime.GOOS, runtime.GOARCH,
    runtime.NumCPU(), runtime.GOMAXPROCS(0), runtime.NumGoroutine())
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -run TestGetRuntimeInfo -v .`
Expected: `--- PASS: TestGetRuntimeInfo`

- [ ] **Step 5: Write the failing test for `getBuildInfo`**

Add to `debug_test.go`:

```go
func TestGetBuildInfo(t *testing.T) {
  info := getBuildInfo()
  if !strings.Contains(info, "github.com/cloudfra/gomain") {
    t.Errorf("expected build info to contain the main module path\n%s", info)
  }
}
```

- [ ] **Step 6: Run it to verify it fails to compile**

Run: `go test -run TestGetBuildInfo -v .`
Expected: FAIL — `undefined: getBuildInfo`

- [ ] **Step 7: Implement `getBuildInfo`**

Add to `debug.go`. Add `"runtime/debug"` and `"strings"` to its imports (note: the import identifier is `debug`, same as this file's name — that's fine, file names aren't Go identifiers and there's no symbol collision):

```go
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
```

- [ ] **Step 8: Run test to verify it passes**

Run: `go test -run TestGetBuildInfo -v .`
Expected: `--- PASS: TestGetBuildInfo`

- [ ] **Step 9: Commit**

```bash
git add debug.go debug_test.go
git commit -m "Add runtime and build info helpers for debug dump"
```

---

### Task 3: Add memory stats & process info helpers

**Files:**

- Modify: `debug.go`
- Test: `debug_test.go`
- [ ] **Step 1: Write the failing test for `getMemoryStats`**

Add to `debug_test.go`:

```go
func TestGetMemoryStats(t *testing.T) {
  stats := getMemoryStats()
  for _, want := range []string{"HeapAlloc", "HeapSys", "NumGC"} {
    if !strings.Contains(stats, want) {
      t.Errorf("expected memory stats to contain %q\n%s", want, stats)
    }
  }
}
```

- [ ] **Step 2: Run it to verify it fails to compile**

Run: `go test -run TestGetMemoryStats -v .`
Expected: FAIL — `undefined: getMemoryStats`

- [ ] **Step 3: Implement `getMemoryStats`**

Add to `debug.go` (add `"time"` to its imports):

```go
func getMemoryStats() string {
  var m runtime.MemStats
  runtime.ReadMemStats(&m)
  return fmt.Sprintf(
    "HeapAlloc: %d bytes\nHeapSys: %d bytes\nHeapObjects: %d\nStackInuse: %d bytes\n"+
      "NumGC: %d\nTotalGCPause: %s\nLastGC: %s\nGCCPUFraction: %f\n",
    m.HeapAlloc, m.HeapSys, m.HeapObjects, m.StackInuse,
    m.NumGC, time.Duration(m.PauseTotalNs), time.Unix(0, int64(m.LastGC)), m.GCCPUFraction)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -run TestGetMemoryStats -v .`
Expected: `--- PASS: TestGetMemoryStats`

- [ ] **Step 5: Write the failing test for `getProcessInfo`**

Add to `debug_test.go` (add `"fmt"` and `"os"` to its imports):

```go
func TestGetProcessInfo(t *testing.T) {
  info := getProcessInfo()
  pid := fmt.Sprintf("%d", os.Getpid())
  for _, want := range []string{pid, exePath()} {
    if !strings.Contains(info, want) {
      t.Errorf("expected process info to contain %q\n%s", want, info)
    }
  }
}
```

- [ ] **Step 6: Run it to verify it fails to compile**

Run: `go test -run TestGetProcessInfo -v .`
Expected: FAIL — `undefined: getProcessInfo` (and `undefined: processStartTime`)

- [ ] **Step 7: Implement `processStartTime` and `getProcessInfo`**

Add to `debug.go` (add `"os"` to its imports). Place the package-level var near the top of the file, after the `import` block:

```go
var processStartTime = time.Now()

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
```

- [ ] **Step 8: Run test to verify it passes**

Run: `go test -run TestGetProcessInfo -v .`
Expected: `--- PASS: TestGetProcessInfo`

- [ ] **Step 9: Commit**

```bash
git add debug.go debug_test.go
git commit -m "Add memory stats and process info helpers for debug dump"
```

---

### Task 4: Add sensitive info helper (args & environment)

**Files:**

- Modify: `debug.go`
- Test: `debug_test.go`
- [ ] **Step 1: Write the failing test**

Add to `debug_test.go`:

```go
func TestGetSensitiveInfo(t *testing.T) {
  info := getSensitiveInfo()
  for _, want := range []string{"Args:", "Environment:", os.Args[0]} {
    if !strings.Contains(info, want) {
      t.Errorf("expected sensitive info to contain %q\n%s", want, info)
    }
  }
}
```

- [ ] **Step 2: Run it to verify it fails to compile**

Run: `go test -run TestGetSensitiveInfo -v .`
Expected: FAIL — `undefined: getSensitiveInfo`

- [ ] **Step 3: Implement `getSensitiveInfo`**

Add to `debug.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -run TestGetSensitiveInfo -v .`
Expected: `--- PASS: TestGetSensitiveInfo`

- [ ] **Step 5: Commit**

```bash
git add debug.go debug_test.go
git commit -m "Add sensitive info helper (args and environment) for debug dump"
```

---

### Task 5: Assemble the full dump (`getDebugDump`/`logDebugDump`)

**Files:**

- Modify: `debug.go`
- Test: `debug_test.go`
- [ ] **Step 1: Write the failing tests**

Add to `debug_test.go`:

```go
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
```

- [ ] **Step 2: Run them to verify they fail to compile**

Run: `go test -run 'TestGetDebugDump|TestLogDebugDump' -v .`
Expected: FAIL — `undefined: getDebugDump` / `undefined: logDebugDump`

- [ ] **Step 3: Implement `getDebugDump` and `logDebugDump`**

Add to `debug.go`:

```go
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
  log.Printf("%s", string(getDebugDump(cfg)))
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run 'TestGetDebugDump|TestLogDebugDump' -v .`
Expected: all three `--- PASS`

- [ ] **Step 5: Run the full `debug.go` test suite to make sure nothing regressed**

Run: `go test -run 'TestGetStackDump|TestGetRuntimeInfo|TestGetBuildInfo|TestGetMemoryStats|TestGetProcessInfo|TestGetSensitiveInfo|TestGetDebugDump|TestGetDebugDumpSensitive|TestLogDebugDump' -v .`
Expected: all `--- PASS`, final `PASS` / `ok`

- [ ] **Step 6: Commit**

```bash
git add debug.go debug_test.go
git commit -m "Assemble extended debug dump from runtime, build, memory, process, and stack info"
```

---

### Task 6: Thread `Config` through signal handling and gate `SIGUSR1`

This is the core behavioral change: `SIGUSR1` on non-Windows now does nothing
unless `cfg.Debug` is `true`, in which case it calls `logDebugDump(cfg)`
instead of the old bare `logStackDump()`. To make this possible, `cfg Config`
must flow from `platformRun` down to `handleSignal`. Because Go build tags mean
only one of `gomain_nonwindows.go`/`gomain_windows.go`/`gomain_plan9.go`/`gomain_js.go`
compiles per platform, but they all implement functions called from the shared
`gomain.go`, **all four must be updated to the same new signatures** even
though only the non-Windows one changes its behavior.

**Files:**

- Modify: `gomain.go:42-73` (`runInteractive`/`runInteractiveInternal`)
- Modify: `gomain_nonwindows.go:25-46` (`platformRun`/`handleSignal`)
- Modify: `gomain_windows.go:35` (`runInteractive(f)` call inside `platformRun`) and `:270-280` (`handleSignal`)
- Modify: `gomain_plan9.go:25-43` (`platformRun`/`handleSignal`)
- Modify: `gomain_js.go:25-37` (`platformRun`/`handleSignal`)
- Modify: `gomain_test.go` (4 call sites)
- [ ] **Step 1: Update `gomain.go` to thread `cfg` through `runInteractive`/`runInteractiveInternal`**

In `gomain.go`, change:

```go
func runInteractive(f MainFunc) {
  sigCh := make(chan os.Signal, 1)
  signal.Notify(sigCh, getTerminalSignals()...)
  defer func() {
    signal.Stop(sigCh)
    close(sigCh)
  }()
  runInteractiveInternal(f, sigCh)
}

func runInteractiveInternal(f MainFunc, sigCh chan os.Signal) {
  mainErrCh := make(chan error, 1)

  mc := internal.NewRunCtx()
  defer mc.Close()

  go func() {
    mainErrCh <- f(mc.Wait)
    close(mainErrCh)
  }()

  select {
  case err := <-mainErrCh:
    handleError(err)
    return
  case sig := <-sigCh:
    if handleSignal(sig) {
      signal.Stop(sigCh)
      mc.Kill()
    }
  }
}
```

to:

```go
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

  select {
  case err := <-mainErrCh:
    handleError(err)
    return
  case sig := <-sigCh:
    if handleSignal(sig, cfg) {
      signal.Stop(sigCh)
      mc.Kill()
    }
  }
}
```

Leave `handleSignalBase(sig os.Signal) bool` exactly as-is — it only handles
`SIGINT`/`SIGKILL`, neither of which is gated by `Debug`, so it doesn't need
`cfg`.

- [ ] **Step 2: Update `gomain_nonwindows.go` — gate `SIGUSR1` behind `cfg.Debug`**

Change:

```go
func platformRun(f MainFunc, cfg Config) {
  runInteractive(f)
}
```

to:

```go
func platformRun(f MainFunc, cfg Config) {
  runInteractive(f, cfg)
}
```

And change:

```go
func handleSignal(sig os.Signal) bool {
  switch sig {
  case syscall.SIGTERM:
    return true
  case syscall.SIGABRT:
    logStackDump()
    return true
  case syscall.SIGUSR1:
    logStackDump()
    return false
  default:
    return handleSignalBase(sig)
  }
}
```

to:

```go
func handleSignal(sig os.Signal, cfg Config) bool {
  switch sig {
  case syscall.SIGTERM:
    return true
  case syscall.SIGABRT:
    logStackDump()
    return true
  case syscall.SIGUSR1:
    if cfg.Debug {
      logDebugDump(cfg)
    }
    return false
  default:
    return handleSignalBase(sig)
  }
}
```

Note `SIGUSR1` always returns `false` (the process keeps running) whether or
not `Debug` is enabled — only whether anything gets *logged* changes. This
preserves today's "the process survives `SIGUSR1`" guarantee and avoids the OS
default disposition (terminate) that would kick in if we stopped registering
the signal.

- [ ] **Step 3: Update `gomain_windows.go` — match the new signature (no behavior change)**

Change:

```go
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
```

to:

```go
func handleSignal(sig os.Signal, cfg Config) bool {
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
```

Also change the `runInteractive(f)` call inside `platformRun` to
`runInteractive(f, cfg)`:

```go
func platformRun(f MainFunc, cfg Config) {
  svcMode, err := svc.IsWindowsService()
  if err != nil {
    log.Fatalf("failed to determine if we are running in service: %v", err)
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
```

- [ ] **Step 4: Update `gomain_plan9.go` — match the new signature (no behavior change)**

Change:

```go
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
```

to:

```go
func platformRun(f MainFunc, cfg Config) {
  runInteractive(f, cfg)
}

func getTerminalSignals() []os.Signal {
  return append(getTerminalSignalsBase(), syscall.SIGTERM, syscall.SIGABRT)
}

func handleSignal(sig os.Signal, cfg Config) bool {
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
```

- [ ] **Step 5: Update `gomain_js.go` — match the new signature (no behavior change)**

Change:

```go
func platformRun(f MainFunc, cfg Config) {
  runInteractive(f)
}

func getTerminalSignals() []os.Signal {
  return append(getTerminalSignalsBase(), syscall.SIGTERM)
}

func handleSignal(sig os.Signal) bool {
  switch sig {
  case syscall.SIGTERM:
    return true
  default:
    return handleSignalBase(sig)
  }
}
```

to:

```go
func platformRun(f MainFunc, cfg Config) {
  runInteractive(f, cfg)
}

func getTerminalSignals() []os.Signal {
  return append(getTerminalSignalsBase(), syscall.SIGTERM)
}

func handleSignal(sig os.Signal, cfg Config) bool {
  switch sig {
  case syscall.SIGTERM:
    return true
  default:
    return handleSignalBase(sig)
  }
}
```

- [ ] **Step 6: Try building — confirm the compile errors point at the test call sites**

Run: `go vet .`
Expected: FAIL, with errors like:

```text
./gomain_test.go:69:14: not enough arguments in call to handleSignal
./gomain_test.go:88:36: not enough arguments in call to runInteractiveInternal
./gomain_test.go:111:36: not enough arguments in call to runInteractiveInternal
./gomain_test.go:131:30: not enough arguments in call to runInteractive
./gomain_test.go:135:21: not enough arguments in call to Run
```

(Exact line numbers may differ slightly — fix whatever `go vet` points at.)

Note: `Run(mainFunc, Config{})` itself doesn't need changes — `Run` already
takes a `Config` and forwards it to `platformRun`. `go vet` lists it above only
because the error cascades from `runInteractive`'s new signature inside it —
double check this one actually still compiles as `Run(mainFunc, Config{})`
once the others are fixed; if it does, leave it untouched.

- [ ] **Step 7: Fix the call sites in `gomain_test.go`**

In `TestHandleSignalBase`, change:

```go
      got := handleSignal(tc.input)
```

to:

```go
      got := handleSignal(tc.input, Config{})
```

In `TestRunInteractiveInternal`, change:

```go
  runInteractiveInternal(mainFunc, sigCh)
```

to:

```go
  runInteractiveInternal(mainFunc, sigCh, Config{})
```

In `TestRunInteractiveInternalAllSignals`, change:

```go
  runInteractiveInternal(mainFunc, sigCh)
```

to:

```go
  runInteractiveInternal(mainFunc, sigCh, Config{})
```

In `TestRunInteractiveAllSignals`, change:

```go
  runInteractive(mainFunc)
```

to:

```go
  runInteractive(mainFunc, Config{})
```

(`Run(mainFunc, Config{})` in the same test stays as-is.)

- [ ] **Step 8: Run `go vet` again to confirm everything compiles**

Run: `go vet .`
Expected: no output, exit code 0

- [ ] **Step 9: Run the full existing signal-handling test suite to confirm no regressions**

Run: `go test -run 'TestHandleSignalBase|TestRunInteractiveInternal|TestRunInteractiveInternalAllSignals|TestRunInteractiveAllSignals' -v .`
Expected: all subtests `--- PASS`, including `TestHandleSignalBase/user_defined_signal_1` (SIGUSR1) showing `want: false` still satisfied — because with `Config{}` (i.e. `Debug: false`), `handleSignal` now ignores `SIGUSR1` and returns `false`, the same return value as before, just without logging a stack dump.

- [ ] **Step 10: Commit**

```bash
git add gomain.go gomain_nonwindows.go gomain_windows.go gomain_plan9.go gomain_js.go gomain_test.go
git commit -m "Thread Config through signal handling and gate SIGUSR1 dump behind Config.Debug"
```

---

### Task 7: Add dedicated tests for the `SIGUSR1`/`Debug` gating behavior

**Files:**

- Modify: `gomain_nonwindows_test.go`

- [ ] **Step 1: Write the new gating tests**

Add to `gomain_nonwindows_test.go` (it already imports `"os"` and `"syscall"`):

```go
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
```

This exercises all three configurations of `handleSignal(syscall.SIGUSR1, ...)`
end-to-end (including the `logDebugDump` call when `Debug` is `true`),
confirming: (a) it never crashes, and (b) `SIGUSR1` never terminates the
process regardless of `Debug`/`DebugSensitive` — only whether a dump gets
logged changes.

- [ ] **Step 2: Run the test**

Run: `go test -run TestHandleSignalSIGUSR1Gating -v .`
Expected: all three subtests `--- PASS`

- [ ] **Step 3: Commit**

```bash
git add gomain_nonwindows_test.go
git commit -m "Add tests for SIGUSR1 gating behavior under Config.Debug/DebugSensitive"
```

---

### Task 8: Update README and run the full test suite

**Files:**

- Modify: `README.md`

- [ ] **Step 1: Update the "Dump Stack Trace" section**

In `README.md`, the current section reads:

```markdown
## Dump Stack Trace

Any application that uses this library on non-Windows OSes can dump a stack trace via:

```bash
# Get the process ID.
ps -a

kill -s SIGUSR1 [PID]
```

```text

Replace it with:

```markdown
## Debug Dump

Applications that opt in via `gomain.Config{Debug: true}` can trigger an
extended debug dump on non-Windows OSes by sending `SIGUSR1`:

```bash
# Get the process ID.
ps -a

kill -s SIGUSR1 [PID]
```

> **Note:** `SIGUSR1` only produces output when `Config.Debug` is `true`. If
> `Debug` is `false` (the default), the signal is caught and ignored — the
> process keeps running but nothing is logged. This is a change from prior
> versions, where `SIGUSR1` always logged a basic stack trace.

The dump includes:

- Go runtime info (version, GOOS/GOARCH, NumCPU, GOMAXPROCS, goroutine count)
- Build info (main module, dependencies, VCS revision when available)
- Memory and GC stats
- Process info (PID, executable path, working directory, hostname, uptime)
- The full goroutine stack trace

Setting `gomain.Config{Debug: true, DebugSensitive: true}` additionally
includes the process's command-line arguments and environment variables in
the dump. Because these can contain secrets (API keys, tokens, passwords
passed via flags or env vars), `DebugSensitive` is a separate opt-in from
`Debug` — only enable it if you're confident the dump's destination (e.g. your
log storage) is appropriately access-controlled.

```text

- [ ] **Step 2: Run the entire test suite to confirm everything passes together**

Run: `go test ./... -v 2>&1 | tail -100`
Expected: every package reports `ok` and the tail shows overall `PASS`. Skim
for any `--- FAIL` lines — there should be none.

- [ ] **Step 3: Run `go vet` one more time across the whole module**

Run: `go vet ./...`
Expected: no output, exit code 0

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "Document Config.Debug/DebugSensitive and the extended SIGUSR1 debug dump"
```

---

## Final verification checklist

- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` succeeds with no warnings
- [ ] `go test ./... -v` — all tests pass, including the new `TestGetRuntimeInfo`,
      `TestGetBuildInfo`, `TestGetMemoryStats`, `TestGetProcessInfo`,
      `TestGetSensitiveInfo`, `TestGetDebugDump`, `TestGetDebugDumpSensitive`,
      `TestLogDebugDump`, and `TestHandleSignalSIGUSR1Gating`
- [ ] `kill -s SIGUSR1 [PID]` against a locally-run example app
      (`go run ./cmd/example`) with `Config{Debug: true}` set logs the full
      extended dump; with `Config{}` (default) it logs nothing and the process
      keeps running — confirm by manually editing `cmd/example/example.go`'s
      `Config{}` to `Config{Debug: true}` temporarily, running it, sending the
      signal, observing the log output, then reverting the temporary edit
      (don't commit it)

# Extended SIGUSR1 Debug Dump

Date: 2026-06-08
Status: Approved

## Problem

Today, sending `SIGUSR1` to a `gomain`-managed process always logs a bare
goroutine stack dump (`debug.go: getStackDump`/`logStackDump`), unconditionally,
on non-Windows platforms. This is useful but minimal — it's missing a lot of
context that would help diagnose a running process: Go runtime/build details,
memory and GC stats, process identity, etc.

We want to extend the `SIGUSR1` dump with this additional runtime information,
while being careful that some of it (environment variables, command-line args)
can leak secrets and shouldn't be logged unconditionally.

## Scope

- Non-Windows (`!windows && !plan9 && !js`) only. SIGUSR1 doesn't exist on
  Windows/Plan 9/JS, and there is currently no equivalent live-trigger
  mechanism on those platforms. Adding one (e.g. `SIGBREAK` or a custom
  Windows Service control code) is explicitly out of scope for this change —
  it can be designed and added separately later.
- `Config` gains two new fields (`Debug`, `DebugSensitive`) that are part of
  the shared, cross-platform `Config` struct (so the API is forward
  compatible), but only the non-Windows signal handler currently acts on them.

## Design

### Config additions

```go
type Config struct {
    ServiceName        string
    ServiceDescription string
    Command            string
    Debug              bool // opt-in: enables the extended SIGUSR1 debug dump
    DebugSensitive     bool // opt-in: additionally include args/env in the dump (only takes effect alongside Debug)
}
```

### Gating behavior (breaking change vs. today)

`SIGUSR1` remains registered in `getTerminalSignals()` on non-Windows — if we
stopped registering it, the OS's default disposition for `SIGUSR1` is to
terminate the process, which would be a worse regression than today's
always-on dump.

In `handleSignal`, the `SIGUSR1` case becomes conditional on `cfg.Debug`:

- `cfg.Debug == false` (the default): the signal is caught and silently
  ignored. No dump is produced; the process keeps running. **This changes
  today's behavior** — previously `SIGUSR1` always produced a basic stack
  dump. Going forward, callers must opt in with `Config{Debug: true}` to get
  any dump from `SIGUSR1`. This needs to be called out in the README.
- `cfg.Debug == true`: runs the new extended debug dump (see below).
- `cfg.DebugSensitive == true` additionally appends the sensitive-info
  section to that dump. It has no effect unless `Debug` is also `true`.

`SIGABRT`/`SIGKILL` crash-time dumps are **untouched** — they keep calling the
existing plain `logStackDump()` regardless of `Debug`/`DebugSensitive`, since
they're unconditional post-mortem aids for an unexpected termination, not the
opt-in "give me a debug dump while running" feature that `SIGUSR1` represents.

### Threading `cfg` through signal handling

`cfg Config` needs to reach `handleSignal` so it can decide what to do with
`SIGUSR1`. This means updating signatures along the call chain:

- `runInteractive(f MainFunc)` → `runInteractive(f MainFunc, cfg Config)`
- `runInteractiveInternal(f MainFunc, sigCh chan os.Signal)` →
  `runInteractiveInternal(f MainFunc, sigCh chan os.Signal, cfg Config)`
- `handleSignal(sig os.Signal) bool` → `handleSignal(sig os.Signal, cfg Config) bool`

`handleSignal` is implemented per-platform (`gomain_nonwindows.go`,
`gomain_windows.go`, `gomain_plan9.go`, `gomain_js.go`), all sharing one call
site in `gomain.go`, so all four implementations get the new parameter for
signature consistency — only the non-Windows variant branches on
`cfg.Debug`/`cfg.DebugSensitive`; the others simply ignore it. The shared
`handleSignalBase(sig os.Signal) bool` keeps its current signature unchanged —
it only handles `SIGINT`/`SIGKILL`, neither of which is gated by `Debug`, so
there's no reason to thread `cfg` into it.

`getTerminalSignals()` is unchanged — `SIGUSR1` stays registered
unconditionally.

### Content of the extended debug dump

When `SIGUSR1` fires and `cfg.Debug == true`, `logDebugDump(cfg)` logs a
single block composed of these sections, in order:

1. **Runtime** — `runtime.Version()`, `runtime.GOOS`/`runtime.GOARCH`,
   `runtime.NumCPU()`, `runtime.GOMAXPROCS(0)`, `runtime.NumGoroutine()`
2. **Build info** — via `runtime/debug.ReadBuildInfo()`: main module path +
   version, Go version used to build, VCS revision/time/dirty flag,
   dependency list (`path@version`)
3. **Memory & GC** — via `runtime.ReadMemStats()`: heap alloc/sys/objects,
   stack in-use, `NumGC`, total/last GC pause, GC CPU fraction
4. **Process info** — PID (`os.Getpid()`), executable path (reusing the
   existing `exePath()` from `util.go`), working directory (`os.Getwd()`),
   hostname (`os.Hostname()`), process start time and uptime (tracked via a
   new package-level `var processStartTime = time.Now()` in `debug.go`)
5. **Sensitive** *(only when `cfg.DebugSensitive == true`)* — `os.Args` and
   `os.Environ()`. This section exists specifically because dumping
   environment variables or full command-line arguments unconditionally risks
   leaking secrets (API keys, tokens, passwords passed via flags/env) into
   logs, so it must be explicitly opted into beyond just `Debug`.
6. **Goroutine stack dump** — the existing `getStackDump()` output, appended
   last, exactly as today's `SIGUSR1` handler produces it.

Format: plain text, simple section headers, concatenated into one buffer and
emitted via a single `log.Printf("%s", ...)`, consistent with how
`logStackDump` works today. No structured (JSON) format — this keeps the
output greppable in logs and simple to substring-test, matching the existing
`TestGetStackDump` style.

### Function decomposition (`debug.go`)

Existing `getStackDump`/`logStackDump` remain unchanged (still used directly
by the `SIGABRT`/`SIGKILL` crash paths). New additions:

- `var processStartTime = time.Now()`
- `getRuntimeInfo() string`
- `getBuildInfo() string`
- `getMemoryStats() string`
- `getProcessInfo() string`
- `getSensitiveInfo() string`
- `getDebugDump(cfg Config) []byte` — assembles sections 1–4 (and 5 if
  `cfg.DebugSensitive`), then appends `getStackDump()`
- `logDebugDump(cfg Config)` — logs the assembled dump

Each builder function returns a plain `string` and has a single, narrow
responsibility — independently readable and testable, mirroring the existing
`getStackDump`/`logStackDump` split.

## Testing

- `debug_test.go`: one substring-check test per new builder function, e.g.:
  - runtime info contains `runtime.Version()`'s output
  - build info contains the module path (`github.com/jeremyje/gomain`)
  - memory stats contains `"HeapAlloc"`
  - process info contains the current PID
  - sensitive info contains a known env var or `os.Args[0]`
  - `getDebugDump`/`logDebugDump`: verify the sensitive section is present
    only when `DebugSensitive: true`, and absent when it's `false`, and that
    nothing crashes — same lightweight style as today's `TestGetStackDump`
- `gomain_nonwindows_test.go` / `gomain_test.go`: update
  `handleSignalTestCases` and call sites for the new `handleSignal(sig, cfg)`
  signature (defaulting to `Config{}` so `SIGUSR1` keeps returning
  `false`/ignored in the generic table-driven cases), plus a dedicated case
  exercising `SIGUSR1` with `Config{Debug: true}` (and one with
  `Config{Debug: true, DebugSensitive: true}`) to ensure the extended path
  runs without crashing and still returns `false` (process keeps running).

## Documentation

Update the README's "Dump Stack Trace" section to:

- Document `Config.Debug` / `Config.DebugSensitive`
- Describe the expanded dump content
- Call out that `SIGUSR1` now requires `Debug: true` to produce any output —
  this is a behavior change from the previously always-on stack dump

## Out of scope

- Any Windows (or Plan 9 / JS) equivalent live-trigger mechanism for the
  extended dump (e.g. `SIGBREAK`, custom Windows Service control codes, an
  HTTP/file-based trigger). `Config.Debug`/`Config.DebugSensitive` exist in
  the shared `Config` struct for forward compatibility, but no platform other
  than the existing non-Windows `SIGUSR1` path acts on them yet.
- Programmatic/non-signal access to the debug dump (e.g. an exported function
  apps could call directly).

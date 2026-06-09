# gomain

A main harness for Go applications that run for a long time. This harness supports:

* Run as Windows Service.
* Handle OS signals gracefully.
* Basic debug signal handling.

## Usage

```go
import (
  "github.com/jeremyje/gomain"
)

func main() {
  gomain.Run(runServer, gomain.Config{
    ServiceName:        "App",
    ServiceDescription: "App does stuff.",
    Command:            *flagValue,
  })
}

func runServer(wait func()) error {
  server := New()
  go func() {
    wait()
    // Terminates the server and causes Run to complete.
    server.Shutdown()
  }()
  return server.Run()
}

```

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

* Go runtime info (version, GOOS/GOARCH, NumCPU, GOMAXPROCS, goroutine count)
* Build info (main module, dependencies, VCS revision when available)
* Memory and GC stats
* Process info (PID, executable path, working directory, hostname, uptime)
* The full goroutine stack trace

Setting `gomain.Config{Debug: true, DebugSensitive: true}` additionally
includes the process's command-line arguments and environment variables in
the dump. Because these can contain secrets (API keys, tokens, passwords
passed via flags or env vars), `DebugSensitive` is a separate opt-in from
`Debug` — only enable it if you're confident the dump's destination (e.g. your
log storage) is appropriately access-controlled.

## 1. State package

- [x] 1.1 Create an internal state package (e.g. `internal/app/daemon`) with a `State` value object holding PID, StartTime (`time.Time`), Namespace, Labels, Delay (`time.Duration`), Max (`int`), and LogPath; no global state.
- [x] 1.2 Add an exported path resolver `DefaultStatePath()` (and a log path helper) under `os.UserCacheDir()`+`/kubectl-notify/`, plus a `Store` type constructed with an injected file path so tests can use a temp dir.
- [x] 1.3 Implement `Store.Read() (State, bool, error)` (JSON decode; `false` when absent), `Store.Write(State) error` (creating the dir, atomic-ish write), and `Store.Remove() error` (idempotent, ignore `os.ErrNotExist`).
- [x] 1.4 Implement an `Alive(pid int) bool` helper (e.g. `Signal(syscall.Signal(0))`) and a `State.Uptime(now)` helper.

## 2. State package tests

- [x] 2.1 Test `Store` round-trip: write then read returns the same `State`; read of a missing file returns `found=false` with no error.
- [x] 2.2 Test `Remove` is idempotent (no error when the file is absent) and that a written file is gone after `Remove`.
- [x] 2.3 Test `Alive` returns true for the current process PID and false for an unused/dead PID, and `Uptime` computes `now - StartTime`.

## 3. Detach / re-exec helper

- [x] 3.1 Add a re-exec helper that resolves `os.Executable`, builds an `exec.Command` re-running `watch` with the same namespace/labels/`--delay`/`--max` (without `--background`) plus a hidden state-file flag, redirects child stdout/stderr to the log file, and sets a `SysProcAttr` that detaches the child into its own session/process group.
- [x] 3.2 Isolate the platform-specific `SysProcAttr` (Unix `Setsid`) behind a build-tagged helper (e.g. `detach_unix.go`) so the build stays portable; keep non-Unix best-effort.
- [x] 3.3 Have the helper start the child, return its PID (and the log path), and not wait on it.

## 4. Watch command --background branch

- [x] 4.1 Add a `--background` (bool) flag and a hidden `--__daemon-statefile` (string) flag to `newWatchCommand`; keep the hidden flag out of `--help`.
- [x] 4.2 In `RunE`, when `--background` is set: read the state file; if a recorded watcher is alive, print "already running (PID N); run `kubectl notify stop` first" to `ErrOut`, return a non-nil error (non-zero exit); otherwise proceed.
- [x] 4.3 On a clear-to-start background run: re-exec the detached child, write the `State` (PID, now, namespace, labels, delay, max, log path), print the running PID and log path to `Out`, and return nil (exit 0).
- [x] 4.4 When the hidden state-file flag is set (the child path), run the normal foreground pipeline and, on return/exit, `Remove` that state file (idempotent) so a clean exit clears state.

## 5. Status and stop commands

- [x] 5.1 Create `cmd/status.go` with `newStatusCommand(streams)` returning the `status` cobra command (with `--help`); register it in `cmd/root.go`.
- [x] 5.2 Implement `status` RunE: read state; if absent or PID not alive, print "no background watch is running", `Remove` stale state, exit 0; if alive, print running PID, namespace (or "all namespaces"), labels (or "none"), delay, max, uptime, and log path; exit 0.
- [x] 5.3 Create `cmd/stop.go` with `newStopCommand(streams)` returning the `stop` cobra command (with `--help`); register it in `cmd/root.go`.
- [x] 5.4 Implement `stop` RunE: read state; if absent or PID not alive, print "no background watch is running", `Remove` stale state, exit 0; if alive, send SIGTERM to the PID, `Remove` the state file, print confirmation, exit 0.

## 6. Command tests

- [x] 6.1 Test the single-instance guard: with a `Store` pointing at a temp file holding a live (current-process) PID, the `--background` start path is refused with a non-zero error; with a stale/dead PID it is not refused.
- [x] 6.2 Test `status` rendering against a temp `Store`: running state prints PID/filter/delay/max/uptime/log path; missing/stale state prints "no background watch is running" and clears the file; both exit 0.
- [x] 6.3 Test `stop` against a temp `Store`: dead/absent PID is a clean no-op that clears state and exits 0 (avoid signalling a real foreign process in tests).

## 7. Verification

- [x] 7.1 Run `make lint build test` (with `-race`) and ensure all pass.
- [x] 7.2 Confirm `kubectl notify watch --help` documents `--background` (and hides the internal flag), and that `kubectl notify status --help` and `kubectl notify stop --help` exist and describe the commands.
- [x] 7.3 Manually verify the round trip on the dev machine: `watch --background` starts and returns, `status` shows it running with details, `stop` terminates it and `status` then reports nothing running.

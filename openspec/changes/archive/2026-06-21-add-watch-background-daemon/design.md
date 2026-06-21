## Context

`kubectl notify watch` (in `cmd/watch.go`) resolves a `*rest.Config`, builds the
k8s events adapter, the desktop notifier, and the `internal/app/controller`,
derives a SIGINT/SIGTERM-cancellable context via `signal.NotifyContext`, starts
`go controller.Run(ctx)`, then blocks in `source.Watch(ctx, filter, ctrl)`. On
signal it cancels, the controller does a final flush, and the command exits `0`.

This works only while the launching terminal stays open. We want the same
pipeline to keep running detached, plus a way to inspect and stop it. A Go
program cannot `fork()` cleanly (the runtime is multi-threaded), so "go to the
background" means **re-exec**: the process launches a fresh copy of its own
binary as a detached child and exits. The child runs the identical foreground
pipeline. Lifecycle coordination (is one running? which PID? stop it) happens
through a small on-disk state file.

Constraints from AGENTS.md: no global state (inject dependencies), wrap errors
with package context, keep the new package free of cobra/adapter types where it
can, alias port imports with a `Port` suffix, every subcommand has `--help`, and
the assistant does not run git.

## Goals / Non-Goals

**Goals:**
- A `--background` flag on `watch` that detaches via re-exec and returns the
  parent immediately with exit `0`, leaving a child running the existing
  pipeline.
- Exactly one background watcher at a time, enforced via the state file.
- A `status` command reporting running/stopped, PID, filter, delay, max, uptime,
  and log path.
- A `stop` command that SIGTERMs the recorded PID (clean final flush via the
  existing handler) and clears state, reporting cleanly when none is running.
- A small, injectable state package owning the file location and read/write/remove.

**Non-Goals:**
- Multiple/named concurrent watchers (single watcher only; explicitly chosen).
- A long-lived supervisor that restarts a crashed child, health checks, or log
  rotation. The child is a plain detached process; if it dies, `status` reports
  it stale.
- An OS service integration (launchd/systemd unit generation) — out of scope.
- Changing the foreground `watch`, the controller, or the ports/adapters.

## Decisions

**1. Detach by re-executing the binary as a detached child.**
The parent (the `--background` invocation) resolves its own executable
(`os.Executable`), and re-runs it as the foreground `watch` for the same flags,
minus `--background`, with stdout/stderr redirected to a log file and a
`SysProcAttr` that starts a new session/process group (`Setsid: true` on
Unix) so it survives the parent terminal. The child is started with
`exec.Command`; the parent records the child PID, writes the state file, prints
a confirmation, and returns nil (exit `0`). The child itself takes the normal
foreground path — it does not know or care that it was launched detached, except
that on a clean exit it removes the state file it was told to own.
*How the child knows it is the detached worker:* the parent passes a hidden flag
(e.g. `--__daemon-statefile=<path>`) so the child, after running the pipeline,
removes that file on exit. The flag is hidden from `--help`.
*Alternative:* a true double-fork daemon — rejected; not idiomatic or safe in
the Go runtime. *Alternative:* require the user to run under `nohup &`/systemd —
rejected per the chosen "self-detaching" decision; less turnkey.

**2. Single watcher enforced through a state file.**
A single JSON state file in a per-user dir (`os.UserCacheDir()` +
`/kubectl-notify/watch.json`, falling back as needed) records: PID, start time
(RFC3339), namespace, labels, delay, max, and log path. `watch --background`
first checks for an existing state file whose PID is **alive**; if so it errors
("a background watch is already running (PID N); run `kubectl notify stop`
first") and exits non-zero. A state file whose PID is **not alive** is treated as
stale and overwritten. Liveness is checked with `os.FindProcess` +
`Signal(syscall.Signal(0))` on Unix.
*Alternative:* a lock file or a running socket — rejected as heavier than needed
for a single desktop watcher.

**3. A small `internal/...` state package, no global state.**
A `Store` type is constructed with the resolved file path (the path resolver is
a separate exported func so tests can point at a temp dir). It exposes
`Read() (State, bool, error)`, `Write(State) error`, `Remove() error`, plus a
`State` value object (the fields above) and an `Alive(pid)`/`Uptime()` helper.
The command layer injects a `Store`; the package imports no cobra or adapter
types. This keeps file-location logic in one tested place and obeys the
no-global-state rule.
*Alternative:* inline file handling in `cmd/` — rejected; harder to test and
duplicated across `watch`/`status`/`stop`.

**4. `status` reads and reports; it does not connect to the cluster.**
`status` resolves the state path, reads the file, and: if absent or the PID is
dead, prints "no background watch is running" (clearing a stale file); if alive,
prints PID, namespace (or "all namespaces"), labels (or "none"), delay, max,
uptime (`now - start`), and the log path. Exit `0` in both cases — "not running"
is not an error.

**5. `stop` signals SIGTERM, then removes state.**
`stop` reads the state file; if none/dead, prints "no background watch is
running", removes any stale file, exits `0`. If alive, it sends SIGTERM to the
PID so the child's existing `signal.NotifyContext` handler cancels the context
and the controller performs its final flush; then `stop` removes the state file
and prints confirmation. It does not block waiting for the child to die (best
effort), but it does verify the signal send succeeded.
*Alternative:* SIGKILL — rejected; would skip the final flush. *Alternative:*
wait-and-poll for exit — deferred; SIGTERM + remove is sufficient and simple.

**6. The detached child owns state-file removal on clean exit.**
Because `stop` also removes the file, removal is idempotent (remove-if-exists,
ignore `os.ErrNotExist`). If the child crashes without removing the file, the
liveness check makes the next `status`/`watch --background` treat it as stale.

## Risks / Trade-offs

- [Re-exec uses `os.Executable`, which can resolve oddly if the binary was
  moved/deleted] → Acceptable for a kubectl plugin installed on PATH; surface a
  wrapped error if it fails rather than silently misbehaving.
- [Stale state file after a crash] → Mitigated by PID liveness checks on every
  read; a dead PID is reported as not-running and the file is cleared.
- [PID reuse: a recycled PID could look "alive"] → Low risk for a desktop tool;
  accepted. A future enhancement could also match the start time.
- [`Setsid`/process-group detach is Unix-specific] → The project targets
  operator desktops (macOS/Linux primarily). Windows detach uses a different
  `SysProcAttr`; gate the Unix-specific attr behind build tags or a small
  platform helper so the build stays portable. Keep Windows best-effort.
- [Child logs grow unbounded] → No rotation in scope; documented. The log path
  is surfaced by `status` so the operator can manage it.

## Migration Plan

Additive only: a new flag, two new subcommands, and a new internal package. No
existing behavior changes; foreground `watch`, the controller, and the
ports/adapters are untouched. No persisted data format exists prior to this
change, so there is nothing to migrate; removing the feature is just deleting
the state file. No rollback concerns.

## Open Questions

- None blocking. Windows detach specifics and log rotation are explicitly
  deferred; the Unix path is the primary target.

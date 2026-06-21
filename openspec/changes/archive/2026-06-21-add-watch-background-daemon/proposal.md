## Why

`kubectl notify watch` runs in the foreground and dies when the terminal closes,
so an operator who wants ambient event notifications must keep a terminal open
and dedicated to it. There is also no way to ask "is a watch already running?"
or to stop one without finding and killing the process by hand. Running the
watch as a managed background daemon, with commands to inspect and stop it,
makes it usable as a set-and-forget desktop companion.

## What Changes

- Add a `--background` flag to `kubectl notify watch`. With it, the command
  re-launches its own binary as a **detached child process** (its own session,
  stdio redirected to a log file), records the child's PID and run parameters in
  a state file, prints where it is running, and the parent returns immediately
  (exit `0`). Without `--background`, `watch` behaves exactly as today
  (foreground, blocks until SIGINT).
- Enforce a **single background watcher**: starting `watch --background` while
  one is already running fails with a clear message telling the operator to
  `stop` the existing one first. `status` and `stop` therefore take no target.
- Add a `kubectl notify status` subcommand that reports detailed state of the
  background watcher: running vs. stopped, PID, the active filter
  (namespace/labels), the `--delay`/`--max` values, uptime, and the log file
  path.
- Add a `kubectl notify stop` subcommand that signals the recorded background
  watcher to terminate (SIGTERM, so the existing handler performs its clean
  final flush) and removes the state file. When no watcher is running it reports
  that cleanly and exits `0`.
- Persist watcher state (PID, start time, namespace, labels, delay, max, log
  path) in a single state file under a per-user state directory, managed by a
  small internal package with no global state.

## Capabilities

### New Capabilities
- `watch-background-daemon`: Running `kubectl notify watch` as a detached,
  single-instance background process via `--background`, including the re-exec
  detach mechanics, the persisted watcher state file, and the lifecycle behavior
  exposed by the new `status` and `stop` subcommands.

### Modified Capabilities
<!-- None: the foreground watch, the controller, and the ports/adapters are
     unchanged. The --background flag adds a new branch in the watch command but
     does not alter existing foreground behavior. -->

## Impact

- New internal package (e.g. `internal/app/daemon` or `internal/daemonstate`)
  owning the state file location, read/write/remove, and the detach/re-exec and
  signal helpers; no global state, dependencies injected.
- `cmd/watch.go` gains a `--background` branch that, in the parent, re-execs and
  writes state; the detached child runs the existing foreground pipeline and, on
  exit, removes its state file.
- New `cmd/status.go` and `cmd/stop.go` subcommands registered on the root
  command in `cmd/root.go`.
- A state file written under the user's state/cache directory
  (`os.UserCacheDir`/XDG) in a `kubectl-notify` subdirectory, plus a log file
  for the detached child's output.
- No new third-party dependencies; reuses the existing controller and adapters.
- No changes to the `EventSource`/`Notifier` ports or the foreground `watch`
  contract.

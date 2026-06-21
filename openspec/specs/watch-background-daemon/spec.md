# watch-background-daemon Specification

## Purpose
TBD - created by archiving change add-watch-background-daemon. Update Purpose after archive.
## Requirements
### Requirement: Background flag runs the watch as a detached daemon

The `kubectl notify watch` command SHALL accept a `--background` flag. When the
flag is set, the command SHALL launch a detached child process that runs the
identical foreground watch pipeline (same namespace, labels, `--delay`, and
`--max`), with the child's standard output and error redirected to a log file
and the child placed in its own session/process group so it survives the parent
terminal. After starting the child, the parent process SHALL record the
watcher's state, print where the watch is running and its log path, and exit
with code `0` without blocking. When `--background` is not set, the command
SHALL behave exactly as the existing foreground watch.

#### Scenario: Background flag detaches and returns immediately

- **WHEN** a user runs `kubectl notify watch --background`
- **THEN** a detached child process is started running the watch pipeline, the parent prints the running PID and log path and exits with code `0`, and the watch continues after the parent returns

#### Scenario: Foreground watch is unchanged

- **WHEN** a user runs `kubectl notify watch` without `--background`
- **THEN** the command runs in the foreground and blocks until interrupted, exactly as before

#### Scenario: Detached child cleans up its state on exit

- **WHEN** the detached background watch exits cleanly
- **THEN** it removes the state file it was assigned so subsequent `status`/`stop` report no running watch

### Requirement: Only one background watcher runs at a time

The system SHALL allow at most one background watcher at a time, tracked by a
single state file in a per-user state directory. Before starting a new
background watch, the command SHALL check whether a recorded watcher is still
alive; if one is alive it SHALL refuse to start a second, print a message
directing the user to stop the existing watcher first, and exit with a non-zero
status. A recorded watcher whose process is no longer alive SHALL be treated as
stale and overwritten rather than blocking a new start.

#### Scenario: Second background watch is refused while one is running

- **WHEN** a user runs `kubectl notify watch --background` while a live background watch already exists
- **THEN** the command does not start a second watcher, prints a message telling the user to run `kubectl notify stop` first, and exits non-zero

#### Scenario: Stale state does not block a new watch

- **WHEN** a user runs `kubectl notify watch --background` and the recorded watcher's process is no longer alive
- **THEN** the stale state is replaced and a new background watch starts normally

### Requirement: Status reports the background watcher state

The system SHALL provide a `kubectl notify status` subcommand that reports the
state of the background watcher without connecting to the cluster. When a live
watcher exists, status SHALL report that it is running along with its PID, the
filter (namespace or "all namespaces", and labels or "none"), the `--delay` and
`--max` values, the uptime derived from the recorded start time, and the log
file path. When no live watcher exists, status SHALL report that no background
watch is running and SHALL clear any stale state. The command SHALL exit with
code `0` whether or not a watcher is running.

#### Scenario: Status reports a running watcher in detail

- **WHEN** a user runs `kubectl notify status` while a background watch is running
- **THEN** the command prints that it is running with the PID, namespace/labels filter, delay, max, uptime, and log file path, and exits with code `0`

#### Scenario: Status reports when nothing is running

- **WHEN** a user runs `kubectl notify status` and no live background watch exists
- **THEN** the command prints that no background watch is running, clears any stale state, and exits with code `0`

### Requirement: Stop terminates the background watcher cleanly

The system SHALL provide a `kubectl notify stop` subcommand that stops the
background watcher. When a live watcher exists, stop SHALL send a termination
signal (SIGTERM) to the recorded process so that the watcher's existing signal
handler performs its clean final flush, SHALL remove the state file, and SHALL
print confirmation. When no live watcher exists, stop SHALL report that no
background watch is running, remove any stale state, and exit with code `0`.

#### Scenario: Stop signals and clears a running watcher

- **WHEN** a user runs `kubectl notify stop` while a background watch is running
- **THEN** the command sends SIGTERM to the recorded PID, removes the state file, prints confirmation, and exits with code `0`

#### Scenario: Stop is a clean no-op when nothing is running

- **WHEN** a user runs `kubectl notify stop` and no live background watch exists
- **THEN** the command reports that no background watch is running, clears any stale state, and exits with code `0`


## Context

The codebase follows a hexagonal (ports & adapters) architecture. Two ports exist:

- Input: `internal/port/datasources/events` — `EventSource.Watch(ctx, Filter, Observer)`
  pushes each `events.Event` to an `Observer.OnEvent(ctx, Event)`.
- Output: `internal/port/notification` — `Notifier.Notify(ctx, Notification)`.

Concrete adapters exist for both (`k8s` events, `desktop` toasts). The only thing
missing is the application logic that joins them and a command to run it live. The
`test` subcommand already demonstrates the wiring style: construct an adapter in the
command's `RunE`, call the port, report errors to `streams.ErrOut`.

Constraints from AGENTS.md: no global state (inject dependencies via constructors),
wrap errors with package context, the controller and command must stay free of
adapter-specific types where the ports already abstract them, and a port import must
be aliased with a `Port` suffix.

## Goals / Non-Goals

**Goals:**
- A controller in the application layer that implements `events.Observer`, maps each
  `events.Event` to a `notification.Notification`, and delivers it via `Notifier`.
- A deterministic, documented event→notification mapping (title, body, urgency).
- A time-windowed debounce: events are buffered for a configurable window and
  flushed together, so bursts do not produce one toast per event.
- Burst summarisation: when a window holds more than a configurable threshold of
  events, emit one summary notification per object kind + reason
  (`"<COUNT> events of <KIND>/<REASON>"`) instead of individual notifications.
- A `kubectl notify watch` command that composes k8s adapter → controller → desktop
  adapter and runs until SIGINT.
- Per-event delivery failures are tolerated: log and continue rather than aborting.

**Non-Goals:**
- No new event sources (FluxCD) or notification sinks (web UI, Slack) — those are
  separate changes that reuse this controller unchanged.
- No per-object deduplication or token-bucket rate-limiting beyond the time-window
  debounce described here (future enhancement).
- No persistence or replay of missed events.

## Decisions

**1. Controller implements `Observer` and owns a buffer + background flush loop.**
The `EventSource.Watch` owns the watch goroutine and reconnect logic and calls
`OnEvent` synchronously on that goroutine. `OnEvent` therefore does no delivery work:
it appends the mapped event to an in-memory slice guarded by a `sync.Mutex` and
returns immediately. A separate `Run(ctx)` method runs a `time.Ticker` at the
debounce window and, on each tick, swaps out the buffer under the lock and flushes
it. The watch command starts `go controller.Run(ctx)` before calling
`source.Watch(ctx, filter, controller)`; when `ctx` is cancelled `Run` performs a
final flush of any buffered events and returns. This keeps `OnEvent` non-blocking
(a slow notifier no longer stalls the watch) and confines all delivery to one
goroutine, so the `Notifier` is never called concurrently.
*Alternative:* a stateless `OnEvent` that delivers inline — rejected because it
cannot debounce and lets a slow toast stall the watch stream.
*Alternative:* a per-event resettable timer (true debounce) — rejected as more
complex; a fixed tumbling window is simpler and matches "wait X seconds, then send".

**2. Mapping rules live in the controller (one place, well-specified).**
- Title: `"<Kind>/<Name>: <Reason>"` (object identity + reason), falling back
  gracefully when fields are empty.
- Body: the event `Message`.
- Urgency: `Type == "Warning"` → `UrgencyCritical`; `Type == "Normal"` (or unknown)
  → `UrgencyNormal`. This is the only event field that is semantically a severity.
*Alternative:* configurable templates — deferred; start with a fixed, tested mapping.

**3. Flush logic: individual below threshold, summary-by-kind/reason above it.**
On each window flush the controller takes the buffered events and decides by count:
- If `len(buffer) <= threshold`: deliver each event as its own notification using the
  mapping from decision #2 (preserves full detail when volume is manageable).
- If `len(buffer) > threshold`: group events by the `(Kind, Reason)` pair — the
  *cause* of the events, not their severity — and for each distinct group deliver one
  summary notification with body `"<count> events of <Kind>/<Reason>"`
  (e.g. `"12 events of Pod/FailedScheduling"`). The summary's urgency is critical if
  any event in the group is a `Warning`, otherwise normal. This bounds the number of
  toasts per window to the number of distinct kind/reason groups.
The window (default e.g. 5s) and threshold (default e.g. 10) are injected via the
constructor and surfaced as command flags. A window of `0` disables buffering
(flush on every event) so the feature is opt-out.
*Alternative:* group by `Type` (Normal/Warning) — rejected by design feedback; the
operator wants to know *what* is happening (the kind/reason), not just its severity.
*Alternative:* always summarise, never send individual events — rejected; for low
volume the operator wants the actual message, not a count.

**4. Delivery failure policy: log and continue.**
`Observer.OnEvent` returning a non-nil error stops the watch (per the port
contract). A failed toast must NOT kill a long-running watch, so the controller
swallows the `Notifier` error after logging it to an injected `io.Writer`
(`streams.ErrOut`) and returns nil. *Alternative:* propagate the error and stop —
rejected; one transient notification failure shouldn't end the session.

**5. `watch` command mirrors `test` for wiring and flag handling.**
It resolves `*rest.Config` from the shared `configFlags`, builds the k8s adapter,
builds the desktop adapter, and constructs the controller with the notifier,
`streams.ErrOut`, and the debounce window (`--delay`, a `time.Duration`) and batch
threshold (`--max`, an `int`). It builds the `Filter` from `--namespace` (via
configFlags) and a new `--labels` flag, derives a context cancelled on SIGINT
(`signal.NotifyContext`), starts `go controller.Run(ctx)`, then calls
`source.Watch(ctx, filter, controller)`. Errors are printed to `ErrOut`; clean
cancellation exits 0.

## Risks / Trade-offs

- [A slow `Notify` stalls the watch] → Mitigated: `OnEvent` only buffers and
  returns; delivery happens on the `Run` goroutine, so a slow notifier delays the
  next flush but never blocks the watch stream.
- [Unbounded buffer growth if events arrive far faster than the window flushes]
  → The threshold path collapses a window to one notification per type, so toast
  volume is bounded; the in-memory slice still holds one window's events, which is
  acceptable for a desktop tool. A future bound/drop policy can cap the slice.
- [Log-and-continue can hide a persistently broken notifier] → Every failure is
  written to stderr, so the operator sees the noise; not silent.
- [Event floods produce notification floods] → Out of scope here; dedup/rate-limit
  is a follow-up change that wraps this controller.

## Migration Plan

Additive only: new package and new subcommand. No existing behavior changes, no
rollback concerns. `test` and the existing ports/adapters are untouched.

## Open Questions

- None blocking. Configurable mapping templates and event throttling are explicitly
  deferred to later changes.

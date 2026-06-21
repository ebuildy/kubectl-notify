## 1. Controller package

- [x] 1.1 Create `internal/app/controller` package importing only `eventsPort` and `notificationPort` (with `Port`-suffixed aliases).
- [x] 1.2 Define a `Controller` struct holding a `notificationPort.Notifier`, an `io.Writer` for diagnostics, a `time.Duration` window, an `int` threshold, a `sync.Mutex`, and an event buffer; add a `New(notifier, logOut, window, threshold)` constructor (no global state).
- [x] 1.3 Add a compile-time assertion that `*Controller` satisfies `eventsPort.Observer`.
- [x] 1.4 Implement `mapEvent(events.Event) notification.Notification`: title from kind/name + reason (graceful when empty), body from message, urgency `Warning`→`UrgencyCritical` else `UrgencyNormal`.
- [x] 1.5 Implement `OnEvent(ctx, Event)`: append the event to the buffer under the lock and return nil without delivering. When window is 0, flush immediately instead of buffering.

## 2. Debounce and flush

- [x] 2.1 Implement `Run(ctx)`: a `time.Ticker` at the window; on each tick call `flush`; on `ctx.Done()` call `flush` once more and return.
- [x] 2.2 Implement `flush`: swap out the buffer under the lock, then deliver outside the lock so `Notify` is never called while holding the mutex.
- [x] 2.3 Flush decision: when `len(batch) <= threshold` deliver each event via `mapEvent`; when `len(batch) > threshold` group by the `(Kind, Reason)` pair and deliver one summary notification per group with body `"<count> events of <Kind>/<Reason>"` and urgency critical if any event in the group is `Warning`, else normal.
- [x] 2.4 On any `Notify` error during flush, write a diagnostic line to the log writer and continue delivering the rest of the batch.

## 3. Controller tests

- [x] 3.1 Add a fake `Notifier` capturing the notifications it receives (and one that returns an error).
- [x] 3.2 Test mapping: `Warning`→critical, `Normal`/unknown→normal, title contains identity+reason, body equals message.
- [x] 3.3 Test buffering: `OnEvent` does not call `Notify`; events are delivered only after a flush.
- [x] 3.4 Test threshold behaviour: at/below threshold → one notification per event; above threshold → one `"<count> events of <Kind>/<Reason>"` summary per distinct group, zero individual notifications, and summary urgency critical when the group contains a `Warning`.
- [x] 3.5 Test final flush: cancelling `Run`'s context delivers remaining buffered events exactly once.
- [x] 3.6 Test delivery failure: a `Notify` error is logged and the remaining batch is still delivered.

## 4. Watch command

- [x] 4.1 Create `cmd/watch.go` with `newWatchCommand(streams)` returning the `watch` cobra command; register it in `cmd/root.go`.
- [x] 4.2 Add `--labels` (string), `--delay` (duration, default e.g. 5s), and `--max` (int, default e.g. 10) flags; resolve namespace from `configFlags`.
- [x] 4.3 Build the `events.Filter` from the resolved namespace (`namespace` key when set) and `--labels` (`labels` key when set); empty filter when neither set.
- [x] 4.4 In `RunE`: resolve `*rest.Config`, construct the k8s adapter, the desktop adapter, and the controller (notifier, `streams.ErrOut`, `--delay`, `--max`).
- [x] 4.5 Derive a context cancelled on SIGINT via `signal.NotifyContext`; start `go controller.Run(ctx)`, then call `source.Watch(ctx, filter, controller)`.
- [x] 4.6 Report `Watch` errors to `streams.ErrOut` and return them (non-zero exit); return nil on clean cancellation (exit 0).

## 5. Verification

- [x] 5.1 Add a unit test for the filter-building helper covering namespace-only, labels-only, both, and neither.
- [x] 5.2 Run `make lint build test` (with `-race` to catch buffer data races) and ensure all pass.
- [x] 5.3 Confirm `kubectl notify watch --help` documents `--labels`, `--delay`, and `--max`.

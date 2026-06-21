## Why

The plugin now has both sides of the hexagon: an `EventSource` input port (with a
Kubernetes adapter) and a `Notifier` output port (with a desktop adapter). Nothing
connects them — there is no application logic that consumes events and turns them
into notifications, and no user-facing command that runs a live watch. Without this
bridge, the plugin can only send a static test toast; it cannot do its core job.

## What Changes

- Introduce a **controller** in the application layer that observes the
  `EventSource` port, maps each `events.Event` into a `notification.Notification`,
  and delivers it through the `Notifier` port. The controller depends only on the
  two ports, never on concrete adapters.
- Define the event→notification mapping: title from the object identity and reason,
  body from the message, and urgency derived from the event type (`Warning` →
  critical, everything else → normal/low).
- **Debounce and batch events**: the controller buffers incoming events for a
  configurable window (X seconds) before notifying, instead of firing one toast per
  event. When few events arrive in a window they are sent individually; when too many
  arrive (above a configurable threshold) the controller collapses them into a
  summary grouped by the event cause (object kind + reason) —
  `"<COUNT> events of <KIND>/<REASON>"` — instead of flooding the desktop with
  individual toasts.
- Add a `kubectl notify watch` subcommand that wires the Kubernetes events adapter
  and the desktop adapter through the controller, honoring the standard kubectl
  connection flags plus `--namespace`/`--labels` filtering, exposing the debounce
  window and batch threshold as flags, and running until the user cancels
  (Ctrl-C / SIGINT).
- Decide controller behaviour when a single notification fails to deliver: log and
  continue the watch rather than tearing down the stream.

## Capabilities

### New Capabilities
- `event-notification-controller`: The application-layer bridge that subscribes to
  the `EventSource` port as an `Observer`, maps events to notifications, and
  forwards them through the `Notifier` port, including the mapping rules, the
  time-windowed debounce/batch behaviour with per-type summarisation, and the
  per-event delivery-failure policy.
- `watch-command`: The `kubectl notify watch` CLI subcommand that composes the
  Kubernetes adapter, the controller, and the desktop adapter into a running
  pipeline driven by the standard kubectl flags, a filter, and the
  debounce-window / batch-threshold flags.

### Modified Capabilities
<!-- None: the EventSource and Notifier ports are unchanged; this change only consumes them. -->

## Impact

- New package `internal/app` (or similar) holding the controller, its mapping
  logic, and a time-windowed buffer with a background flush loop; depends only on
  `internal/port/datasources/events` and `internal/port/notification`.
- New `cmd/watch.go` subcommand registered on the root command in `cmd/root.go`.
- No new third-party dependencies; reuses existing adapters and `client-go` config
  resolution from `genericclioptions.ConfigFlags`.
- No changes to the `EventSource` or `Notifier` port contracts.

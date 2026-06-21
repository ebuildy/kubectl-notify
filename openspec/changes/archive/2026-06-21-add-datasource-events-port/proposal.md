## Why

`kubectl-notify` can already deliver notifications (the output side of the hexagon),
but it has no way to ingest the events that should trigger them. We need the input
side: a technology-agnostic port that streams events from a source so that later a
controller can turn them into notifications. Kubernetes events are the first source
operators care about.

## What Changes

- Add a new **input port** `EventSource` that watches a source, subject to a
  `Filter`, and forwards each matching event to a registered `Observer` (Observer
  pattern, see ADR-001), blocking until the context is cancelled or the stream ends.
- Define an `Event` value object carried through the port (technology-agnostic:
  no Kubernetes or transport types leak through the interface).
- Define a generic `Filter` (`map[string]string`) passed to `Watch`; each adapter
  translates the recognized keys into its own native filtering and rejects keys it
  does not support.
- Add the first adapter implementing the port: a **Kubernetes events** watcher that
  uses `client-go` to watch `core/v1` Events, maps them onto the port's `Event`, and
  translates the `Filter` (e.g. `namespace`, `labels`) into Kubernetes watch options.
- Respect standard kubectl connection flags (`--kubeconfig`, `--context`,
  `--namespace`) when constructing the Kubernetes adapter.
- Out of scope (later change): the controller that wires the `EventSource` to the
  `Notifier`, and any CLI command exposing it.

## Capabilities

### New Capabilities
- `datasource-events`: the input port for watching a source of events and forwarding
  them to a handler, plus the Kubernetes events adapter that implements it.

### Modified Capabilities
<!-- None: the desktop-notification capability is unchanged. -->

## Impact

- New package `internal/port/datasources/events` (the `EventSource` port + `Event` type).
- New package `internal/adapter/datasources/k8s` (the Kubernetes events adapter).
- New dependencies promoted from indirect to direct: `k8s.io/client-go`,
  `k8s.io/apimachinery`, `k8s.io/api` (already in `go.sum` via `cli-runtime`).
- No CLI or controller wiring yet; no breaking changes to existing behavior.

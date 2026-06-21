## Why

`kubectl-notify` is currently an empty bootstrap with no way to surface anything to the user. Before wiring up Kubernetes/FluxCD event watchers, we need the output side of the hexagon in place: a stable, technology-agnostic way to emit a notification. Starting with desktop notifications (toasts) gives us a vertical slice that proves the hexagonal structure end-to-end and is independently testable.

## What Changes

- Introduce the hexagonal (ports & adapters) skeleton: `internal/port/` for interfaces and `internal/adapter/` for implementations.
- Define a `Notifier` **port** (interface) for sending a notification with a title, body, and severity/urgency level.
- Add a **desktop adapter** that implements `Notifier` by showing an OS desktop notification (macOS / Linux / Windows) via a cross-platform Go library.
- Add a small `notify test` cobra subcommand that sends a sample notification through the port so the adapter can be exercised from the CLI.
- Add a new `go.mod` dependency for the desktop notification library (justified in `design.md`).

## Capabilities

### New Capabilities
- `desktop-notification`: Sending a user-facing desktop notification (toast) through a technology-agnostic notifier port, with a desktop adapter implementation and a CLI command to exercise it.

### Modified Capabilities
<!-- None — this is the first capability in the project. -->

## Impact

- New packages: `internal/port/notification/`, `internal/adapter/notification/desktop/`.
- New CLI subcommand: `kubectl notify test` in `cmd/`.
- New dependency: a cross-platform desktop notification library (e.g. `github.com/gen2brain/beeep`).
- No breaking changes — net-new behavior on an empty bootstrap.

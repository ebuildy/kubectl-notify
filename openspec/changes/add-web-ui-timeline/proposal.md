## Why

Desktop toasts are ephemeral and easy to miss: once a notification disappears there is no
history, no overview, and no way to scan what just happened across the cluster. Ops need a
live, glanceable view of recent events. A local web UI — a full-page vertical timeline of
event cards, newest on top, grouped into columns — gives that overview without sending any data
off the machine.

## What Changes

- Add a new `kubectl notify web` subcommand that starts a local HTTP server, opens the
  default browser at its URL, and streams cluster events to the page in real time.
- Add a web server adapter that implements the existing `events.Observer` port: it keeps a
  bounded ring buffer of recent events and fans each new event out to connected browsers.
- Serve a single self-contained, embedded (`//go:embed`) front-end: a full-page vertical
  timeline (most recent events on top, vertical scroll only, usable in a window as narrow as
  ~200px) where events are distributed horizontally into columns by group (reason, kind). Each
  event renders as a card, with the card border colored by urgency level and the namespace and
  labels shown as chips. (Intentionally a simple first pass — a fuller visual design will follow
  later.)
- Expose a small HTTP/WebSocket API: a snapshot endpoint for the buffered history on page
  load and a WebSocket endpoint that pushes each new event as JSON.
- Extend the technology-agnostic `events.Event` value object with a `Timestamp` (for ordering
  the timeline) and an optional `Labels` map (for the card chips); populate them in the
  Kubernetes adapter. These fields are additive and ignored by existing consumers.
- Add a new dependency for the WebSocket transport (justified in `design.md`).

## Capabilities

### New Capabilities
- `web-ui`: the `web` command and the web server adapter — lifecycle (start server, open
  browser, run until interrupted), the HTTP/WebSocket API contract, the bounded event buffer,
  and the timeline rendering rules (vertical, newest-on-top, vertical scroll only, columns by
  group; card per event, border color by urgency, namespace/labels as chips).

### Modified Capabilities
- `datasource-events`: the `Event` value object gains a `Timestamp` and an optional `Labels`
  map, and the Kubernetes adapter populates them. Additive only — existing behavior unchanged.

## Impact

- **New code**: `cmd/web.go` (subcommand), `internal/adapter/web/` (server, ring buffer,
  WebSocket hub, browser-open helper, embedded `static/` assets).
- **Modified code**: `internal/port/datasources/events/events.go` (`Event` fields),
  `internal/adapter/datasources/k8s/k8s.go` (`mapEvent`), `cmd/root.go` (register command).
- **Dependencies**: one new module for WebSocket support (see `design.md`).
- **Network/security**: binds a local HTTP listener (loopback by default); read-only — the UI
  only displays events and never mutates cluster state. No new cluster permissions beyond the
  existing watch.
- **Out of scope**: authentication, TLS, multi-source (FluxCD) cards, and the full visual
  redesign.

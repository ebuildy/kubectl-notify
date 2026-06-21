## Context

`kubectl-notify` is a hexagonal Go plugin. The input port is `events.EventSource`
(`Watch(ctx, filter, Observer)`); the Kubernetes adapter implements it and pushes each event to
an `events.Observer`. The existing `watch` command wires the source to a `Controller` (an
`Observer`) which maps events onto the `Notifier` output port (desktop toasts).

The web UI needs the *rich* event — namespace, kind, name, reason, message, type, timestamp,
labels — not the flattened `Notification` (title/body/urgency). So the natural seam is the
input-side `Observer` port: the web server is just another `Observer`, wired directly to the
source, in parallel to (not replacing) the notification pipeline. No new port is required.

Constraints from `AGENTS.md`: dynamic/client-go connection flags must be honored; read-only
operations only; every new dependency justified here; one change = one responsibility.

## Goals / Non-Goals

**Goals:**
- A `kubectl notify web` command that starts a local server, opens the browser, and runs until
  interrupted (Ctrl-C / SIGTERM), mirroring `watch`'s foreground lifecycle.
- Real-time push of events to the page with a history snapshot on load.
- A self-contained front-end embedded in the binary (no build step, no external CDN).
- Reuse the existing `events.Observer` port and Kubernetes adapter unchanged in spirit (only
  additive `Event` fields).

**Non-Goals:**
- Authentication, TLS, or exposure beyond loopback.
- The final visual design (this is a deliberately minimal first pass).
- Background/daemon mode (`--background`), multi-source cards, or persistence across restarts.

## Decisions

### 1. The web server implements `events.Observer`, wired directly to the source
The `web` command builds the k8s `EventSource` and a `*web.Server`, then calls
`source.Watch(ctx, filter, server)`. The server's `OnEvent` appends to a ring buffer and
broadcasts to connected clients. *Alternative considered:* implement the `Notifier` port and
reuse the controller — rejected because `Notification` discards the structured fields the cards
need, and the debounce/summarise logic actively works against a live timeline.

### 2. Transport: WebSocket via `github.com/coder/websocket`
The data flow is server→client push of JSON events. We use `github.com/coder/websocket`
(formerly `nhooyr.io/websocket`): a single, modern, zero-transitive-dependency module with a
context-aware API that fits the existing `ctx`-driven code. This honors the proposal's stated
preference for WebSocket and keeps the door open for future client→server messages (filtering,
pause). *Alternative considered:* Server-Sent Events over plain `net/http` (zero new
dependencies, browser auto-reconnect) — a strong simpler option, rejected only to match the
requested transport and avoid a later migration; noted as the fallback if the dependency proves
undesirable.

### 3. Front-end: a single embedded static bundle via `//go:embed`
`internal/adapter/web/static/` holds `index.html`, `app.js`, `app.css`, embedded with
`//go:embed static/*` and served from memory. Plain vanilla JS — no framework, no bundler — so
the binary stays self-contained and the "simple first" pass is trivially replaceable later.

### 3a. Layout: vertical timeline, newest-on-top, columns by group
The page is a **vertical** timeline: most recent events at the top, **vertical scroll only**,
and usable in a window as narrow as ~200px. Events are distributed **horizontally into columns
keyed by group (reason + kind)** — all events sharing a group land in the same column, newest
at the top of that column. The columns are laid out with a flex/grid row that wraps or
flex-shrinks so it never forces horizontal scrolling (e.g. `flex-wrap` with min-width columns).
The JS keeps a column index keyed by `reason|kind`, creating a column lazily on first sight of
a group and prepending each new card to its column. *Alternative considered:* a single flat
newest-first list — rejected because grouping by (reason, kind) is the requested organization
and makes bursts of the same cause scannable.

### 4. HTTP/WebSocket API surface
- `GET /` → `index.html`.
- `GET /api/events` → JSON array of the buffered events (newest-last), for initial render.
- `GET /ws` → WebSocket; the server writes one JSON event object per new event.
- `GET /healthz` → `200 ok` (liveness, used by tests).

Event JSON: `{ timestamp, namespace, kind, name, reason, message, type, urgency, labels }`,
where `urgency` is the existing rule (`Warning`→`critical`, else `normal`) computed server-side
so the front-end colors borders without re-deriving policy.

### 5. Bounded ring buffer, fan-out hub
The server keeps the last N events (capacity 100) under a mutex; `/api/events` returns a copy.
A hub holds a set of client channels; `OnEvent` does a non-blocking send to each and drops for
any slow/full client (the client re-syncs via `/api/events` on reconnect) so one stuck browser
never stalls the watch goroutine — consistent with the controller's "never block the watch"
rule.

### 6. Browser open and port selection
`--port` (default `0` = OS-chosen ephemeral port) and `--no-open` (skip launching the browser,
for headless/remote/tests). After the listener binds we know the real URL, print it, then open
it via a small per-OS helper (`open` on darwin, `xdg-open` on linux, `rundll32 url.dll` on
windows). Bind to `127.0.0.1` by default. *Alternative considered:* fixed default port (e.g.
8088) — rejected to avoid collisions; the chosen URL is always printed.

### 7. Additive `Event` fields
Add `Timestamp time.Time` and `Labels map[string]string` to `events.Event`. The k8s `mapEvent`
sets `Timestamp` from `EventTime` (falling back to `LastTimestamp`/`FirstTimestamp`) and
`Labels` from the event object's own metadata labels (no extra API call). Existing consumers
(controller) ignore the new fields, preserving behavior.

## Risks / Trade-offs

- **New dependency for WebSocket** → choose a minimal, well-maintained module
  (`coder/websocket`, no transitive deps) and document SSE as the dependency-free fallback.
- **Slow/stale browser client stalls delivery** → non-blocking fan-out with drop-on-full;
  clients re-sync from `/api/events` on reconnect.
- **Unbounded memory from event volume** → fixed-size ring buffer (capacity 100), oldest
  dropped.
- **Local listener exposure** → bind to loopback only; read-only UI; no cluster mutation; no
  new RBAC beyond the existing watch.
- **Involved-object k8s labels not available without extra GETs** → first pass uses the event
  object's own metadata labels only; richer involved-object labels are out of scope.

## Migration Plan

Purely additive: a new command, a new adapter package, and two additive `Event` fields. No data
migration, no change to existing commands. Rollback = revert the change; nothing persists.

## Open Questions

- Confirm WebSocket (`coder/websocket`) over the simpler dependency-free SSE — proceeding with
  WebSocket per the proposal's stated preference.
- Ring-buffer capacity fixed at 100; a `--max-events`/`--buffer` flag is deferred unless
  needed.

## ADDED Requirements

### Requirement: Web command lifecycle

The system SHALL provide a `kubectl notify web` subcommand that starts a local HTTP server,
opens the user's default browser at the server URL, and streams cluster events to the page
until interrupted. The command MUST honor the standard kubectl connection flags
(`--kubeconfig`, `--context`, `--namespace`) and the `--labels` selector, MUST run in the
foreground and block until the context is cancelled (Ctrl-C / SIGINT or SIGTERM), and MUST
shut the server and the underlying watch down cleanly on exit without leaking goroutines. The
command SHALL expose a `--port` flag (default `0`, meaning an OS-chosen ephemeral port) and a
`--no-open` flag that suppresses launching the browser.

#### Scenario: Server starts and reports its URL

- **WHEN** the user runs `kubectl notify web`
- **THEN** an HTTP server binds to a loopback address, the resolved URL (including the actual
  port) is printed to standard output, and the command keeps running

#### Scenario: Browser is opened by default

- **WHEN** the user runs `kubectl notify web` without `--no-open`
- **THEN** the system attempts to open the resolved URL in the default browser

#### Scenario: Browser open is suppressed

- **WHEN** the user runs `kubectl notify web --no-open`
- **THEN** the server starts and prints its URL but no browser is launched

#### Scenario: Interrupt stops the server cleanly

- **WHEN** the running command receives SIGINT or SIGTERM
- **THEN** the HTTP server and the event watch stop and the command returns without leaking
  goroutines

#### Scenario: Connection error surfaces

- **WHEN** the Kubernetes connection cannot be established
- **THEN** the command prints an error and returns a non-nil error without starting the server

### Requirement: Web server is an event observer

The system SHALL provide a web server adapter that implements the existing `events.Observer`
port and is wired directly to the `EventSource` (not through the notification controller), so
it receives the full `Event` value object. Its `OnEvent` MUST return promptly and MUST NOT
block the watch goroutine on slow or disconnected browser clients.

#### Scenario: Each event reaches the server

- **WHEN** the watched source produces an event
- **THEN** the server's `OnEvent` is invoked with that `Event` and records it for delivery to
  connected clients

#### Scenario: A slow client never stalls the watch

- **WHEN** a connected client cannot keep up with the event rate
- **THEN** delivery to that client is dropped rather than blocking, and `OnEvent` still returns
  promptly for all other clients

### Requirement: Bounded event buffer

The system SHALL keep a bounded, in-memory ring buffer of at most the 100 most recent events.
When the buffer is full, the oldest event MUST be dropped. The buffer MUST be safe for
concurrent access by the watch goroutine and HTTP handlers.

#### Scenario: Recent events are retained

- **WHEN** fewer events than the buffer capacity have been observed
- **THEN** all observed events are retained in observation order

#### Scenario: Oldest events are dropped when full

- **WHEN** more than 100 events have been observed
- **THEN** the buffer retains only the 100 most recent events, dropping the oldest

### Requirement: HTTP and WebSocket API

The system SHALL expose the following endpoints from the web server:
- `GET /` SHALL return the embedded single-page UI.
- `GET /api/events` SHALL return the buffered events as a JSON array in observation order.
- `GET /ws` SHALL upgrade to a WebSocket connection and SHALL write one JSON object per new
  event for the lifetime of the connection.
- `GET /healthz` SHALL return HTTP 200.

Each event JSON object MUST include the timestamp, namespace, kind, name, reason, message,
type, a derived `urgency` (`critical` for a `Warning` type, otherwise `normal`), and labels.

#### Scenario: Snapshot returns buffered history

- **WHEN** a client requests `GET /api/events`
- **THEN** the response is a JSON array of the currently buffered events in observation order

#### Scenario: WebSocket streams new events

- **WHEN** a client is connected to `GET /ws` and a new event is observed
- **THEN** the server writes that event as a JSON object to the WebSocket connection

#### Scenario: Event JSON carries urgency

- **WHEN** an event of type `Warning` is serialized
- **THEN** its JSON `urgency` field is `critical`; for any other type it is `normal`

#### Scenario: Health check responds

- **WHEN** a client requests `GET /healthz`
- **THEN** the server responds with HTTP 200

### Requirement: Timeline rendering

The system SHALL serve a self-contained front-end, embedded in the binary, that renders events
as a full-page vertical timeline with the most recent events at the top. On load it MUST fetch
the buffered history from `/api/events` and then receive new events over the WebSocket. Events
MUST be distributed horizontally into columns by group key (event reason and object kind), so
that all events sharing a group appear in the same column. The layout MUST remain usable in a
narrow window (down to roughly 200px wide) and MUST allow only vertical scrolling — it MUST NOT
require horizontal scrolling. Each event MUST render as a card whose border color reflects its
urgency level, and each card MUST display the event's namespace and labels as chip-style labels
along with its identity (kind/name), reason, and message.

#### Scenario: Cards render newest-first in a vertical timeline

- **WHEN** the page loads with buffered events
- **THEN** each event is shown as a card and the most recent events appear at the top, older
  events below

#### Scenario: Events are grouped into horizontal columns

- **WHEN** events with different group keys (reason, kind) are rendered
- **THEN** events are distributed horizontally into columns by group key, with events sharing a
  group placed in the same column

#### Scenario: Only vertical scrolling, usable when narrow

- **WHEN** the window is resized narrow (down to roughly 200px wide) or the cards exceed the
  viewport height
- **THEN** the timeline remains usable and can be scrolled vertically, and never requires
  horizontal scrolling

#### Scenario: Border color reflects urgency

- **WHEN** a card renders for an event
- **THEN** its border color is determined by the event's urgency level (e.g. a distinct color
  for `critical` versus `normal`)

#### Scenario: Namespace and labels shown as chips

- **WHEN** a card renders for an event that has a namespace and/or labels
- **THEN** the namespace and each label are displayed as chip-style labels on the card

#### Scenario: New events appear live

- **WHEN** the page is open and a new event arrives over the WebSocket
- **THEN** a new card for that event is added at the top of its group column without reloading
  the page

## 1. Extend the events port and Kubernetes adapter

- [ ] 1.1 Add `Timestamp time.Time` and `Labels map[string]string` to `events.Event` in `internal/port/datasources/events/events.go`, documenting them as additive/optional
- [ ] 1.2 Populate the new fields in `mapEvent` in `internal/adapter/datasources/k8s/k8s.go`: `Timestamp` from `EventTime` (fall back to `LastTimestamp`, then `FirstTimestamp`), `Labels` from the event object's metadata labels — no extra API calls
- [ ] 1.3 Add/extend unit tests in `internal/adapter/datasources/k8s/k8s_test.go` asserting timestamp fallback order and label mapping

## 2. Web server adapter

- [ ] 2.1 Create package `internal/adapter/web` with a `Server` type that implements `events.Observer`
- [ ] 2.2 Implement a bounded ring buffer (capacity 100, drop-oldest, mutex-guarded) storing observed events; `OnEvent` appends and returns promptly
- [ ] 2.3 Implement a fan-out hub: register/unregister per-client channels and broadcast each new event with a non-blocking send (drop on full) so a slow client never blocks the watch goroutine
- [ ] 2.4 Define the JSON event DTO `{ timestamp, namespace, kind, name, reason, message, type, urgency, labels }` and a mapper from `events.Event`, deriving `urgency` (`critical` for `Warning`, else `normal`)
- [ ] 2.5 Add the `github.com/coder/websocket` dependency (`go get`) and run `go mod tidy`

## 3. HTTP handlers and embedded UI

- [ ] 3.1 Implement an `http.Handler`/router with `GET /` (embedded UI), `GET /api/events` (JSON snapshot in observation order), `GET /ws` (WebSocket stream of new events), and `GET /healthz` (200)
- [ ] 3.2 Embed `internal/adapter/web/static/*` via `//go:embed` and serve `index.html` from memory
- [ ] 3.3 Create `static/index.html`, `static/app.css`, `static/app.js`: full-page vertical timeline, newest events on top, vertical scroll only, usable down to ~200px wide (no horizontal scroll); events distributed into horizontal columns keyed by group (reason|kind), each column newest-first; on load fetch `/api/events`, then connect `/ws`; render each event as a card with border color by urgency and namespace + labels as chips; prepend each new card to its group column live
- [ ] 3.4 Implement a cross-platform browser-open helper (`open` on darwin, `xdg-open` on linux, `rundll32 url.dll,FileProtocolHandler` on windows)

## 4. The `web` command

- [ ] 4.1 Create `cmd/web.go` with `newWebCommand(streams)`: resolve `*rest.Config` from `configFlags`, build the k8s `EventSource`, build the `web.Server`
- [ ] 4.2 Add `--port` (default 0) and `--no-open` flags; bind an `http.Server` to `127.0.0.1:<port>`, discover the actual address, print the URL, and (unless `--no-open`) open the browser
- [ ] 4.3 Wire `signal.NotifyContext` (SIGINT/SIGTERM); run the HTTP server and `source.Watch(ctx, buildFilter(namespace, labels), server)` concurrently; shut both down cleanly on cancellation without leaking goroutines
- [ ] 4.4 Register the command in `cmd/root.go` via `cmd.AddCommand(newWebCommand(streams))`

## 5. Tests

- [ ] 5.1 Unit-test the ring buffer (retain under capacity, drop-oldest beyond 100) and the event→DTO/urgency mapping
- [ ] 5.2 Unit-test the HTTP handlers with `httptest`: `/healthz` returns 200, `/api/events` returns buffered events as JSON, `/` serves the embedded page
- [ ] 5.3 Test that `OnEvent` does not block when a client channel is full (drop-on-full behavior)
- [ ] 5.4 Add a `cmd/web_test.go` smoke test that builds the command and verifies flags/wiring (mirroring `cmd/watch_test.go`)

## 6. Verification and docs

- [ ] 6.1 Run `make lint build test` and fix any findings
- [ ] 6.2 Manually verify: `kubectl notify web --no-open`, open the printed URL, confirm cards render newest-on-top in vertical columns grouped by (reason, kind) with border colors and namespace/label chips, that the window works at ~200px wide with vertical scroll only, and that new events appear live at the top of their column
- [ ] 6.3 Update `README.md` usage to document the `web` command, then propose a git commit message

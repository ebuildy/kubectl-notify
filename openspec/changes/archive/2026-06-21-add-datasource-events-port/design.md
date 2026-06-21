## Context

The hexagon currently has only its output side: the `notification.Notifier` port
and a desktop adapter. To produce notifications from real activity we need the input
side — a port that produces a stream of events. This change adds that port and the
first adapter (Kubernetes events). The controller that connects the two ports is a
deliberate non-goal here; it lands in a follow-up change.

Constraints (from AGENTS.md): hexagonal layout under `internal/port` and
`internal/adapter`, no global state, errors wrapped at boundaries, `client-go`
dynamic/typed clients for Kubernetes, respect kubectl flags, and paginate/limit list
calls. Port import aliases must be suffixed with `Port`.

## Goals / Non-Goals

**Goals:**
- A technology-agnostic `EventSource` input port with a clean `Event` value object.
- A blocking, handler-callback watch model that the future controller can drive.
- A Kubernetes `core/v1` Events adapter that honors `--kubeconfig/--context/--namespace`.
- Clean shutdown on context cancellation, no goroutine leaks.

**Non-Goals:**
- The controller wiring `EventSource` → `Notifier` (next change).
- A CLI command exposing the watcher.
- FluxCD or other non-Kubernetes adapters (the port is designed to allow them later).
- Event filtering, deduplication, or persistence.

## Decisions

**Port shape: Observer pattern over a returned channel (see ADR-001).**
The `EventSource` is the subject; it pushes each event to a registered `Observer`
via `Watch(ctx context.Context, filter Filter, obs Observer) error`. `Observer` has a single
`OnEvent(ctx, Event) error` method, and an `ObserverFunc` adapter (the
`http.HandlerFunc` idiom) lets callers pass a plain function. The call blocks until
`ctx` is done or the stream ends, matching the user's "thread block to watch
incoming events and forward to a function" intent. Alternatives considered — a
`<-chan Event` channel and a Go 1.23 `iter.Seq2` iterator — are rejected in
[ADR-001](../../../docs/decisions/ADR-001-eventsource-port-observer-vs-channel.md):
a channel pushes lifecycle/error handling onto every caller and complicates
backpressure; the Observer keeps the controller simple, lets the adapter own the
watch loop, and returns a clean terminal `error`. `OnEvent` returning `error` lets a
future controller signal fatal failures to stop the watch.

**`Filter` is a generic `map[string]string` translated per adapter.**
The port stays technology-agnostic by carrying filtering as opaque key/value pairs
(`type Filter map[string]string`) rather than a typed struct with Kubernetes notions.
Each adapter owns the vocabulary it understands — the Kubernetes adapter recognizes
`namespace` (watched namespace) and `labels` (label selector) — and **returns an
error for any unrecognized key** so a typo or unsupported filter fails loudly instead
of silently matching every event (a silent no-op filter is dangerous for a
notification tool). Alternative: a typed `Filter` struct. Rejected — it would either
leak adapter-specific fields into the port or force a lowest-common-denominator shape;
the generic map keeps the port stable while each adapter evolves its own keys. A nil
or empty `Filter` means no filtering.

**`Event` is a flat struct of primitives.** Fields: `Reason`, `Message`, `Type`
(string), and an embedded/inline object reference (`Namespace`, `Kind`, `Name`).
No `k8s.io/...` types in the port. Alternative: pass the raw `*corev1.Event`.
Rejected — it would leak Kubernetes into the port and break the hexagon contract.

**Adapter uses the typed `client-go` Events watch with informer-style resync.**
Use `clientset.CoreV1().Events(ns).Watch(...)` wrapped by a `cache`/retry watcher
(`watchtools.NewRetryWatcher` or a simple reconnect loop) so transient disconnects
don't end the stream. Namespace and label selector come from the `Filter` at watch
time (no `namespace` key ⇒ all namespaces); the adapter is constructed only with the
connection config — a `*rest.Config` (or `genericclioptions.ConfigFlags`) so the
kubectl flags resolve through the standard cli-runtime path. Alternative: dynamic client.
Rejected for v1 — Events have a stable typed schema and the typed client is simpler;
a dynamic variant can come later if CRD event-like resources are needed.

**Package layout.**
- `internal/port/datasources/events/events.go` — `EventSource`, `Observer`,
  `ObserverFunc`, `Event`, `Filter`.
- `internal/adapter/datasources/k8s/k8s.go` — `Adapter`, `New(...)`, compile-time
  `var _ eventsPort.EventSource = (*Adapter)(nil)`.

## Risks / Trade-offs

- **Watch disconnects end the stream silently** → use a retry/reconnect watcher and
  wrap terminal errors with adapter context; surface them via the returned error.
- **`OnEvent` runs on the watch goroutine; a slow observer stalls the stream** →
  documented as caller responsibility for now; the controller's observer can hand off
  to its own buffered channel for slow delivery.
- **Promoting k8s deps from indirect to direct** → they already resolve via
  `cli-runtime`, so no new modules are downloaded; note in `go.mod`.
- **No e2e cluster in CI** → adapter tests use `envtest`/fake clientset per the
  testing rules; the port is tested with a stub source.

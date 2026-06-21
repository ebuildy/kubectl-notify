# ADR-001: EventSource input port uses the Observer pattern over channels

## Status
Accepted

## Date
2026-06-21

## Context
`kubectl-notify` already has the output side of the hexagon (the `Notifier` port and
a desktop adapter). We are adding the input side: a technology-agnostic `EventSource`
port that watches a source of events and forwards them to the application, so that a
later controller can turn events into notifications.

Constraints:
- The port must be technology-agnostic — no Kubernetes, OS, or transport types may
  leak through the interface (hexagonal contract).
- The watch must **block** until the caller's context is cancelled or the stream
  ends, return a **terminal error** wrapped with adapter context, and **not leak
  goroutines**.
- The first adapter is Kubernetes `core/v1` Events; FluxCD and others will follow, so
  the port is a stable implementation target for multiple adapters.
- Downstream work (delivering a desktop notification) may be **slow**, so the design
  must not couple watch throughput to delivery latency.

The open question was how an adapter hands each event to the application: push to a
caller-supplied function/observer, or expose a channel the caller ranges over.

## Decision
Model the port with the **Observer pattern**. The `EventSource` is the subject; it
pushes each `Event` to a registered `Observer`. A function adapter (`ObserverFunc`,
the `http.HandlerFunc` idiom) lets callers pass a plain function where convenient.

```go
// Observer reacts to each event produced by an EventSource.
type Observer interface {
    OnEvent(ctx context.Context, e Event) error
}

// ObserverFunc adapts a plain function to the Observer interface.
type ObserverFunc func(ctx context.Context, e Event) error

func (f ObserverFunc) OnEvent(ctx context.Context, e Event) error { return f(ctx, e) }

// EventSource watches a source and notifies the observer for each event.
// Watch blocks until ctx is cancelled or the stream ends, and returns a
// non-nil error (wrapped with adapter context) on stream failure.
type EventSource interface {
    Watch(ctx context.Context, obs Observer) error
}
```

`OnEvent` runs on the adapter's watch goroutine, in event order.

## Alternatives Considered

### Channel — `Watch(ctx context.Context) (<-chan Event, error)`
- Pros: idiomatic Go for streams; the caller can `select` over the source; natural
  decoupling and buffering; easy fan-in across future sources.
- Cons: **error propagation is clumsy** — a stream that fails mid-flight needs a
  second `<-chan error` or a `<-chan Result{Event, error}` wrapper instead of a plain
  `error` return; **ownership/closure is ambiguous** — who closes the channel on
  cancellation, and the caller must keep draining or leak the producer goroutine.
- Rejected: the error-semantics and ownership cost outweigh the benefits. A caller
  that wants a channel can build one on top of an Observer in three lines; you cannot
  recover clean `error`-return semantics on top of a channel as cleanly.

### Go 1.23 iterator — `Watch(ctx context.Context) iter.Seq2[Event, error]`
- Pros: modern and readable (`for ev, err := range src.Watch(ctx)`); blocks the
  consumer naturally, matching the "thread blocks on the watch" intent.
- Cons: `iter.Seq2` error handling is **convention-only, not enforced**; fan-in is no
  easier than with a callback; less familiar as a contract that several adapters must
  implement identically.
- Rejected: a weaker error contract for a port that is a stable implementation target.

### Bare callback — `Watch(ctx, handler func(Event) error) error`
- Pros: the simplest possible shape.
- Cons: no named domain role; harder to attach observer state/lifecycle; less
  expressive than naming the abstraction.
- Rejected in favor of the Observer interface, which keeps the same ergonomics via
  `ObserverFunc` while giving the role a name and room to grow (e.g. a fan-out
  observer) without changing the port.

## Consequences
- **Clean error flow and lifecycle:** `Watch` returns one wrapped `error`; when it
  returns, the watch is fully stopped — the "clear owner, predictable exit" rule.
- **Slow-observer gotcha:** because `OnEvent` runs on the watch goroutine, a slow
  observer stalls the upstream watch (risking Kubernetes `resourceVersion` expiry).
  The controller's observer should therefore hand events off to its own buffered
  channel and do slow delivery elsewhere. This is documented at the port.
- **Function ergonomics retained:** callers may pass an `ObserverFunc` rather than
  defining a type.
- **Multi-source fan-in:** run each source's `Watch` in its own goroutine under an
  `errgroup`, sharing one `Observer`; the observer must be safe for concurrent
  `OnEvent` calls.
- **Extensible without breaking the port:** if multi-observer dispatch is ever
  needed, a fan-out `Observer` can wrap several observers; the `EventSource`
  interface is unchanged.

Supersedes: none. If a future need (e.g. native multiplexing of many sources at the
port boundary) outweighs these trade-offs, write a new ADR that supersedes this one.

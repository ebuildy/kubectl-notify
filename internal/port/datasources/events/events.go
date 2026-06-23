// Package events defines the technology-agnostic input port for watching a
// source of events and forwarding them to the application. Adapters (Kubernetes
// events, FluxCD, ...) implement the EventSource interface without the port
// knowing about any concrete event technology, transport, or library.
package events

import (
	"context"
	"time"
)

// Event is the value object carried through the EventSource port. Using a
// struct keeps call sites stable as new fields are added over time. All fields
// are primitive, technology-agnostic types so no source-specific detail leaks
// through the port.
type Event struct {
	// Reason is a short, machine-friendly cause of the event.
	Reason string
	// Message is a human-readable description of the event.
	Message string
	// Type categorizes the event, e.g. "Normal" or "Warning".
	Type string
	// Namespace, Kind and Name identify the object the event refers to.
	Namespace string
	Kind      string
	Name      string
	// Timestamp is when the event occurred, used to order a timeline. It is
	// additive and optional: adapters that do not set it leave the zero value,
	// and existing consumers that do not read it are unaffected.
	Timestamp time.Time
	// Labels is an optional set of labels associated with the event. It is
	// additive and optional: a nil map is valid and ignored by consumers that
	// do not read it.
	Labels map[string]string
}

// Filter is a generic, technology-agnostic set of filter keys to values. The
// port carries filtering as opaque key/value pairs so it stays free of any
// concrete event technology; each adapter translates the keys it recognizes
// into its own native filtering mechanism and returns an error for any key it
// does not support, so a misconfigured filter fails loudly rather than silently
// matching everything. A nil or empty Filter means "no filtering".
type Filter map[string]string

// Observer reacts to each event produced by an EventSource. OnEvent runs on the
// adapter's watch goroutine, in event order; returning a non-nil error signals
// the source to stop the watch.
type Observer interface {
	OnEvent(ctx context.Context, e Event) error
}

// ObserverFunc adapts a plain function to the Observer interface (the
// http.HandlerFunc idiom), so callers can pass a function where convenient.
type ObserverFunc func(ctx context.Context, e Event) error

// OnEvent calls f(ctx, e).
func (f ObserverFunc) OnEvent(ctx context.Context, e Event) error { return f(ctx, e) }

// EventSource is the input port for watching a source of events, subject to a
// Filter, and forwarding each matching event to the registered Observer.
//
// Watch blocks the calling goroutine until ctx is cancelled or the underlying
// stream ends. It returns nil (or the stream's terminal error) on a clean stop,
// and a non-nil error wrapped with adapter context on stream failure. When it
// returns, the watch is fully stopped and no goroutines are leaked.
//
// Note: OnEvent runs on the watch goroutine, so a slow observer stalls the
// stream. Callers that do slow delivery should hand events off to their own
// buffered channel inside OnEvent.
type EventSource interface {
	Watch(ctx context.Context, filter Filter, obs Observer) error
}

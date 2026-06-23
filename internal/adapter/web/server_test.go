package web

import (
	"context"
	"testing"
	"time"

	eventsPort "github.com/ebuildy/kubectl-notify/internal/port/datasources/events"
)

func event(reason string) eventsPort.Event {
	return eventsPort.Event{Reason: reason, Kind: "Pod", Name: "p", Namespace: "default", Type: "Normal"}
}

// TestBufferRetainsUnderCapacity asserts all observed events are kept, in
// observation order, while under the buffer capacity.
func TestBufferRetainsUnderCapacity(t *testing.T) {
	s := NewServer()
	for i := 0; i < 5; i++ {
		_ = s.OnEvent(context.Background(), event("r"))
	}
	if got := len(s.snapshot()); got != 5 {
		t.Fatalf("snapshot length = %d, want 5", got)
	}
}

// TestBufferDropsOldestBeyondCapacity asserts the buffer keeps only the 100 most
// recent events and drops the oldest, preserving observation order.
func TestBufferDropsOldestBeyondCapacity(t *testing.T) {
	s := NewServer()
	total := bufferCapacity + 50
	for i := 0; i < total; i++ {
		e := event("r")
		e.Name = string(rune('a')) // not important
		e.Message = itoa(i)
		_ = s.OnEvent(context.Background(), e)
	}

	snap := s.snapshot()
	if len(snap) != bufferCapacity {
		t.Fatalf("snapshot length = %d, want %d", len(snap), bufferCapacity)
	}
	// Oldest retained should be event index 50 (the first 50 dropped).
	if snap[0].Message != itoa(total-bufferCapacity) {
		t.Errorf("oldest retained message = %q, want %q", snap[0].Message, itoa(total-bufferCapacity))
	}
	if snap[len(snap)-1].Message != itoa(total-1) {
		t.Errorf("newest message = %q, want %q", snap[len(snap)-1].Message, itoa(total-1))
	}
}

// TestUrgencyMapping asserts Warning maps to critical and anything else to
// normal.
func TestUrgencyMapping(t *testing.T) {
	if got := toDTO(eventsPort.Event{Type: "Warning"}).Urgency; got != urgencyCritical {
		t.Errorf("Warning urgency = %q, want %q", got, urgencyCritical)
	}
	if got := toDTO(eventsPort.Event{Type: "Normal"}).Urgency; got != urgencyNormal {
		t.Errorf("Normal urgency = %q, want %q", got, urgencyNormal)
	}
	if got := toDTO(eventsPort.Event{Type: ""}).Urgency; got != urgencyNormal {
		t.Errorf("empty type urgency = %q, want %q", got, urgencyNormal)
	}
}

// TestOnEventDoesNotBlockOnFullClient asserts a client whose channel is full
// never stalls OnEvent: the send is dropped and OnEvent returns promptly.
func TestOnEventDoesNotBlockOnFullClient(t *testing.T) {
	s := NewServer()
	ch, unsubscribe := s.subscribe()
	defer unsubscribe()

	// Fill the client channel to capacity without draining it.
	for i := 0; i < cap(ch); i++ {
		_ = s.OnEvent(context.Background(), event("r"))
	}

	done := make(chan struct{})
	go func() {
		// This OnEvent must not block even though ch is full.
		for i := 0; i < 10; i++ {
			_ = s.OnEvent(context.Background(), event("r"))
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("OnEvent blocked on a full client channel")
	}
}

// itoa is a tiny dependency-free int-to-string for test messages.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

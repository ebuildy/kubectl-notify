package events

import (
	"context"
	"errors"
	"testing"
	"time"
)

// stubSource is a minimal EventSource that emits a fixed slice of events to the
// observer and then blocks until the context is cancelled, mimicking a real
// streaming source. It lets us verify the port contract without any concrete
// technology.
type stubSource struct {
	events []Event
}

func (s *stubSource) Watch(ctx context.Context, _ Filter, obs Observer) error {
	for _, e := range s.events {
		if err := obs.OnEvent(ctx, e); err != nil {
			return err
		}
	}
	<-ctx.Done()
	return ctx.Err()
}

// compile-time guarantee that the stub satisfies the port.
var _ EventSource = (*stubSource)(nil)

// TestWatchNotifiesObserverPerEvent verifies the observer's OnEvent is invoked
// once per event, in order.
func TestWatchNotifiesObserverPerEvent(t *testing.T) {
	want := []Event{
		{Reason: "Started", Message: "a", Type: "Normal", Namespace: "ns", Kind: "Pod", Name: "p1"},
		{Reason: "Killing", Message: "b", Type: "Warning", Namespace: "ns", Kind: "Pod", Name: "p2"},
	}
	src := &stubSource{events: want}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var got []Event
	obs := ObserverFunc(func(_ context.Context, e Event) error {
		got = append(got, e)
		if len(got) == len(want) {
			cancel() // stop the watch once all events are delivered
		}
		return nil
	})

	if err := src.Watch(ctx, nil, obs); !errors.Is(err, context.Canceled) {
		t.Fatalf("Watch returned %v, want context.Canceled", err)
	}

	if len(got) != len(want) {
		t.Fatalf("observer saw %d events, want %d", len(got), len(want))
	}
	for i := range want {
		// Event carries a Labels map, so it is not comparable with ==; compare
		// the scalar identity fields that this test sets.
		if got[i].Reason != want[i].Reason || got[i].Message != want[i].Message ||
			got[i].Type != want[i].Type || got[i].Namespace != want[i].Namespace ||
			got[i].Kind != want[i].Kind || got[i].Name != want[i].Name {
			t.Errorf("event %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestWatchBlocksUntilCancel verifies Watch blocks until the context is
// cancelled and then returns.
func TestWatchBlocksUntilCancel(t *testing.T) {
	src := &stubSource{} // no events: Watch should just block on ctx
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- src.Watch(ctx, Filter{}, ObserverFunc(func(context.Context, Event) error { return nil }))
	}()

	select {
	case <-done:
		t.Fatal("Watch returned before context cancellation")
	case <-time.After(50 * time.Millisecond):
	}

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Watch returned %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Watch did not return after context cancellation")
	}
}

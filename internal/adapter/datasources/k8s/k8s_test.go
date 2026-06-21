package k8s_test

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	k8sAdapter "github.com/ebuildy/kubectl-notify/internal/adapter/datasources/k8s"
	eventsPort "github.com/ebuildy/kubectl-notify/internal/port/datasources/events"
)

// Adapter must satisfy the EventSource port (also enforced at compile time in
// the adapter; this keeps the guarantee visible in the test package).
var _ eventsPort.EventSource = (*k8sAdapter.Adapter)(nil)

func newEvent(namespace, name, reason string) *corev1.Event {
	return &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Reason:     reason,
		Message:    "msg-" + reason,
		Type:       "Normal",
		InvolvedObject: corev1.ObjectReference{
			Namespace: namespace,
			Kind:      "Pod",
			Name:      "pod-" + name,
		},
	}
}

// TestWatchForwardsEvents asserts the observer is notified per matching event
// and that each event is mapped onto the port's Event value object.
func TestWatchForwardsEvents(t *testing.T) {
	client := fake.NewSimpleClientset(
		newEvent("default", "e1", "Started"),
		newEvent("default", "e2", "Killing"),
	)
	adapter := k8sAdapter.NewWithClient(client)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	got := make(chan eventsPort.Event, 2)
	obs := eventsPort.ObserverFunc(func(_ context.Context, e eventsPort.Event) error {
		got <- e
		return nil
	})

	done := make(chan error, 1)
	go func() { done <- adapter.Watch(ctx, nil, obs) }()

	want := map[string]bool{"Started": true, "Killing": true}
	for range want {
		select {
		case e := <-got:
			if e.Namespace != "default" || e.Kind != "Pod" {
				t.Errorf("unexpected mapped event: %+v", e)
			}
			delete(want, e.Reason)
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for events; still missing %v", want)
		}
	}
	if len(want) != 0 {
		t.Errorf("did not observe all events, missing %v", want)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Watch returned %v, want nil", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Watch did not return after cancellation")
	}
}

// TestWatchUnsupportedFilterKey asserts an unrecognized filter key fails loudly
// before the watch starts.
func TestWatchUnsupportedFilterKey(t *testing.T) {
	adapter := k8sAdapter.NewWithClient(fake.NewSimpleClientset())

	err := adapter.Watch(context.Background(), eventsPort.Filter{"bogus": "x"},
		eventsPort.ObserverFunc(func(context.Context, eventsPort.Event) error { return nil }))
	if err == nil {
		t.Fatal("expected error for unsupported filter key, got nil")
	}
	if !contains(err.Error(), "bogus") {
		t.Errorf("error %q does not name the unsupported key", err)
	}
}

// TestWatchNamespaceFilter asserts the namespace filter is honored: only events
// in the requested namespace are forwarded.
func TestWatchNamespaceFilter(t *testing.T) {
	client := fake.NewSimpleClientset(
		newEvent("team-a", "e1", "Started"),
		newEvent("team-b", "e2", "Started"),
	)
	adapter := k8sAdapter.NewWithClient(client)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	got := make(chan eventsPort.Event, 2)
	obs := eventsPort.ObserverFunc(func(_ context.Context, e eventsPort.Event) error {
		got <- e
		return nil
	})

	go func() { _ = adapter.Watch(ctx, eventsPort.Filter{"namespace": "team-a"}, obs) }()

	select {
	case e := <-got:
		if e.Namespace != "team-a" {
			t.Errorf("got event from namespace %q, want team-a", e.Namespace)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for team-a event")
	}

	// No team-b event should arrive.
	select {
	case e := <-got:
		if e.Namespace != "team-a" {
			t.Errorf("received event outside team-a: %+v", e)
		}
	case <-time.After(200 * time.Millisecond):
	}
}

// TestWatchContextCancellationStops asserts cancelling the context stops the
// watch and returns cleanly.
func TestWatchContextCancellationStops(t *testing.T) {
	adapter := k8sAdapter.NewWithClient(fake.NewSimpleClientset())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- adapter.Watch(ctx, nil,
			eventsPort.ObserverFunc(func(context.Context, eventsPort.Event) error { return nil }))
	}()

	cancel()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("Watch returned %v, want nil/context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Watch did not return after context cancellation")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

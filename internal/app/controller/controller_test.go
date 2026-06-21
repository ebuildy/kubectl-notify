package controller_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ebuildy/kubectl-notify/internal/app/controller"
	eventsPort "github.com/ebuildy/kubectl-notify/internal/port/datasources/events"
	notificationPort "github.com/ebuildy/kubectl-notify/internal/port/notification"
)

// fakeNotifier captures the notifications it receives. When err is non-nil it
// returns that error for every Notify call (still recording the notification).
type fakeNotifier struct {
	mu  sync.Mutex
	got []notificationPort.Notification
	err error
}

func (f *fakeNotifier) Notify(_ context.Context, n notificationPort.Notification) error {
	f.mu.Lock()
	f.got = append(f.got, n)
	f.mu.Unlock()
	return f.err
}

func (f *fakeNotifier) notifications() []notificationPort.Notification {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]notificationPort.Notification, len(f.got))
	copy(out, f.got)
	return out
}

func (f *fakeNotifier) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.got)
}

// compile-time guarantee that the fake satisfies the Notifier port.
var _ notificationPort.Notifier = (*fakeNotifier)(nil)

// TestMapping verifies the event-to-notification mapping via a zero-window
// controller, which flushes each event immediately.
func TestMapping(t *testing.T) {
	tests := []struct {
		name        string
		event       eventsPort.Event
		wantUrgency notificationPort.Urgency
		wantTitle   string
		wantBody    string
	}{
		{
			name:        "warning maps to critical",
			event:       eventsPort.Event{Kind: "Pod", Name: "p1", Reason: "Failed", Message: "boom", Type: "Warning"},
			wantUrgency: notificationPort.UrgencyCritical,
			wantTitle:   "Pod/p1: Failed",
			wantBody:    "boom",
		},
		{
			name:        "normal maps to normal",
			event:       eventsPort.Event{Kind: "Pod", Name: "p2", Reason: "Started", Message: "ok", Type: "Normal"},
			wantUrgency: notificationPort.UrgencyNormal,
			wantTitle:   "Pod/p2: Started",
			wantBody:    "ok",
		},
		{
			name:        "unknown type maps to normal",
			event:       eventsPort.Event{Kind: "Node", Name: "n1", Reason: "Reboot", Message: "rebooting", Type: "Mystery"},
			wantUrgency: notificationPort.UrgencyNormal,
			wantTitle:   "Node/n1: Reboot",
			wantBody:    "rebooting",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier := &fakeNotifier{}
			c := controller.New(notifier, io.Discard, 0, 10)

			if err := c.OnEvent(context.Background(), tt.event); err != nil {
				t.Fatalf("OnEvent returned %v", err)
			}

			got := notifier.notifications()
			if len(got) != 1 {
				t.Fatalf("got %d notifications, want 1", len(got))
			}
			n := got[0]
			if n.Urgency != tt.wantUrgency {
				t.Errorf("urgency = %v, want %v", n.Urgency, tt.wantUrgency)
			}
			if n.Title != tt.wantTitle {
				t.Errorf("title = %q, want %q", n.Title, tt.wantTitle)
			}
			if n.Body != tt.wantBody {
				t.Errorf("body = %q, want %q", n.Body, tt.wantBody)
			}
		})
	}
}

// TestBufferingDoesNotDeliver verifies OnEvent buffers without delivering when a
// non-zero window is configured; delivery happens only on a flush.
func TestBufferingDoesNotDeliver(t *testing.T) {
	notifier := &fakeNotifier{}
	c := controller.New(notifier, io.Discard, time.Hour, 10)

	for i := 0; i < 3; i++ {
		if err := c.OnEvent(context.Background(), eventsPort.Event{Kind: "Pod", Name: "p", Reason: "R", Type: "Normal"}); err != nil {
			t.Fatalf("OnEvent returned %v", err)
		}
	}

	if got := notifier.count(); got != 0 {
		t.Fatalf("notifier called %d times before flush, want 0", got)
	}

	// Cancelling Run triggers a final flush of the buffered events.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()
	cancel()
	<-done

	if got := notifier.count(); got != 3 {
		t.Fatalf("notifier called %d times after flush, want 3", got)
	}
}

// TestThresholdBelowOrEqual verifies that at or below the threshold each event
// is delivered as its own notification.
func TestThresholdBelowOrEqual(t *testing.T) {
	notifier := &fakeNotifier{}
	c := controller.New(notifier, io.Discard, time.Hour, 3)

	events := []eventsPort.Event{
		{Kind: "Pod", Name: "a", Reason: "Started", Type: "Normal", Message: "m1"},
		{Kind: "Pod", Name: "b", Reason: "Failed", Type: "Warning", Message: "m2"},
		{Kind: "Node", Name: "c", Reason: "Ready", Type: "Normal", Message: "m3"},
	}
	for _, e := range events {
		_ = c.OnEvent(context.Background(), e)
	}

	flushViaCancel(t, c)

	got := notifier.notifications()
	if len(got) != 3 {
		t.Fatalf("got %d notifications, want 3 (one per event)", len(got))
	}
	// Bodies are the original messages, not summaries.
	for _, n := range got {
		if strings.Contains(n.Body, "events of") {
			t.Errorf("expected individual notification, got summary body %q", n.Body)
		}
	}
}

// TestThresholdAbove verifies that above the threshold the window is delivered
// as one summary per distinct (Kind, Reason) group, with no individual
// notifications and critical urgency where a group contains a Warning.
func TestThresholdAbove(t *testing.T) {
	notifier := &fakeNotifier{}
	c := controller.New(notifier, io.Discard, time.Hour, 3)

	// 5 events (> threshold 3) across two groups:
	//   Pod/FailedScheduling x3 (with a Warning) and Node/Ready x2 (all Normal).
	events := []eventsPort.Event{
		{Kind: "Pod", Reason: "FailedScheduling", Type: "Warning"},
		{Kind: "Pod", Reason: "FailedScheduling", Type: "Normal"},
		{Kind: "Pod", Reason: "FailedScheduling", Type: "Normal"},
		{Kind: "Node", Reason: "Ready", Type: "Normal"},
		{Kind: "Node", Reason: "Ready", Type: "Normal"},
	}
	for _, e := range events {
		_ = c.OnEvent(context.Background(), e)
	}

	flushViaCancel(t, c)

	got := notifier.notifications()
	if len(got) != 2 {
		t.Fatalf("got %d notifications, want 2 summaries", len(got))
	}

	byBody := make(map[string]notificationPort.Notification)
	for _, n := range got {
		byBody[n.Body] = n
	}

	podSummary, ok := byBody["3 events of Pod/FailedScheduling"]
	if !ok {
		t.Fatalf("missing Pod summary; got %+v", got)
	}
	if podSummary.Urgency != notificationPort.UrgencyCritical {
		t.Errorf("Pod summary urgency = %v, want critical (group has a Warning)", podSummary.Urgency)
	}

	nodeSummary, ok := byBody["2 events of Node/Ready"]
	if !ok {
		t.Fatalf("missing Node summary; got %+v", got)
	}
	if nodeSummary.Urgency != notificationPort.UrgencyNormal {
		t.Errorf("Node summary urgency = %v, want normal", nodeSummary.Urgency)
	}
}

// TestFinalFlushOnce verifies that cancelling Run delivers buffered events
// exactly once.
func TestFinalFlushOnce(t *testing.T) {
	notifier := &fakeNotifier{}
	c := controller.New(notifier, io.Discard, 50*time.Millisecond, 10)

	_ = c.OnEvent(context.Background(), eventsPort.Event{Kind: "Pod", Name: "p", Reason: "R", Type: "Normal"})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()

	cancel()
	<-done

	// Give any errant extra flush a chance to (incorrectly) fire.
	time.Sleep(100 * time.Millisecond)

	if got := notifier.count(); got != 1 {
		t.Fatalf("notifier called %d times, want exactly 1", got)
	}
}

// TestDeliveryFailureLoggedAndContinues verifies a Notify error is logged and
// the rest of the batch is still delivered.
func TestDeliveryFailureLoggedAndContinues(t *testing.T) {
	var logBuf bytes.Buffer
	notifier := &fakeNotifier{err: errors.New("toast failed")}
	c := controller.New(notifier, &logBuf, time.Hour, 10)

	for i := 0; i < 3; i++ {
		_ = c.OnEvent(context.Background(), eventsPort.Event{Kind: "Pod", Name: "p", Reason: "R", Type: "Normal"})
	}

	flushViaCancel(t, c)

	if got := notifier.count(); got != 3 {
		t.Fatalf("notifier called %d times, want 3 (failure must not abort the batch)", got)
	}
	if !strings.Contains(logBuf.String(), "toast failed") {
		t.Errorf("diagnostic log missing the failure; got %q", logBuf.String())
	}
}

// flushViaCancel runs the controller and immediately cancels it to force a
// final flush of buffered events, then waits for Run to return.
func flushViaCancel(t *testing.T, c *controller.Controller) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after cancellation")
	}
}

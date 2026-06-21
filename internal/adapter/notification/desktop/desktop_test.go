package desktop_test

import (
	"context"
	"testing"

	"github.com/ebuildy/kubectl-notify/internal/adapter/notification/desktop"
	notificationPort "github.com/ebuildy/kubectl-notify/internal/port/notification"
)

// Adapter must satisfy the Notifier port (also enforced at compile time in the
// adapter; this keeps the guarantee visible in the test package).
var _ notificationPort.Notifier = (*desktop.Adapter)(nil)

// TestNotifyDoesNotPanic exercises Notify for every urgency level through the
// Notifier port. On headless CI there is no display, so delivery may return an
// error; that is acceptable. What we assert is that the call returns without
// panicking.
func TestNotifyDoesNotPanic(t *testing.T) {
	var adapter notificationPort.Notifier = desktop.New()

	levels := []notificationPort.Urgency{
		notificationPort.UrgencyLow,
		notificationPort.UrgencyNormal,
		notificationPort.UrgencyCritical,
	}

	for _, level := range levels {
		// Delivery success depends on a desktop session; we only require that
		// the call completes without panicking.
		_ = adapter.Notify(context.Background(), notificationPort.Notification{
			Title:   "kubectl-notify test",
			Body:    "adapter test notification",
			Urgency: level,
		})
	}
}

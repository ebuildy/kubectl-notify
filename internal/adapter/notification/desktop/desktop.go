// Package desktop implements the notification.Notifier port by showing an
// operating-system desktop notification (toast) on macOS, Linux, and Windows.
package desktop

import (
	"context"
	"fmt"

	"github.com/gen2brain/beeep"

	notificationPort "github.com/ebuildy/kubectl-notify/internal/port/notification"
)

// appName is the application name shown on the OS notification.
const appName = "kubectl-notify"

// Adapter delivers notifications as OS desktop toasts via beeep.
type Adapter struct{}

// compile-time guarantee that Adapter satisfies the Notifier port.
var _ notificationPort.Notifier = (*Adapter)(nil)

// New constructs a desktop notification adapter.
func New() *Adapter {
	beeep.AppName = appName
	return &Adapter{}
}

// Notify shows the notification as a desktop toast. Critical notifications use
// an attention-demanding alert; everything else uses a standard toast. Delivery
// errors are wrapped with adapter context.
func (a *Adapter) Notify(_ context.Context, n notificationPort.Notification) error {
	var err error
	switch n.Urgency {
	case notificationPort.UrgencyCritical:
		err = beeep.Alert(n.Title, n.Body, "")
	default:
		err = beeep.Notify(n.Title, n.Body, "")
	}
	if err != nil {
		return fmt.Errorf("desktop notifier: %w", err)
	}
	return nil
}

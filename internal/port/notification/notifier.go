// Package notification defines the technology-agnostic output port for
// surfacing user-facing notifications. Adapters (desktop toast, web UI, Slack,
// ...) implement the Notifier interface without the port knowing about any
// concrete notification technology.
package notification

import "context"

// Urgency describes how attention-demanding a notification is. Adapters map
// these levels onto their underlying platform's notification semantics.
type Urgency int

const (
	// UrgencyLow is informational; it may be shown unobtrusively.
	UrgencyLow Urgency = iota
	// UrgencyNormal is the default level for routine notifications.
	UrgencyNormal
	// UrgencyCritical demands attention and may use an alert/dialog.
	UrgencyCritical
)

// Notification is the value object carried through the Notifier port. Using a
// struct keeps call sites stable as new fields are added over time.
type Notification struct {
	Title   string
	Body    string
	Urgency Urgency
}

// Notifier is the output port for delivering a notification to the user. It is
// intentionally free of any transport, OS, or library detail so that future
// adapters drop in without touching callers.
type Notifier interface {
	Notify(ctx context.Context, n Notification) error
}

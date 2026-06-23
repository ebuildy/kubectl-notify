package web

import (
	"time"

	eventsPort "github.com/ebuildy/kubectl-notify/internal/port/datasources/events"
)

// Urgency levels derived from an event's type and rendered as the card border
// color by the front-end.
const (
	urgencyCritical = "critical"
	urgencyNormal   = "normal"
)

// eventDTO is the JSON shape sent to the browser, both in the /api/events
// snapshot and over the WebSocket. urgency is computed server-side so the
// front-end colors borders without re-deriving policy.
type eventDTO struct {
	Timestamp time.Time         `json:"timestamp"`
	Namespace string            `json:"namespace"`
	Kind      string            `json:"kind"`
	Name      string            `json:"name"`
	Reason    string            `json:"reason"`
	Message   string            `json:"message"`
	Type      string            `json:"type"`
	Urgency   string            `json:"urgency"`
	Labels    map[string]string `json:"labels"`
}

// toDTO maps a port Event onto the JSON DTO, deriving urgency from the event
// type ("Warning" → critical, anything else → normal).
func toDTO(e eventsPort.Event) eventDTO {
	return eventDTO{
		Timestamp: e.Timestamp,
		Namespace: e.Namespace,
		Kind:      e.Kind,
		Name:      e.Name,
		Reason:    e.Reason,
		Message:   e.Message,
		Type:      e.Type,
		Urgency:   urgencyFor(e.Type),
		Labels:    e.Labels,
	}
}

// urgencyFor maps an event type to an urgency level.
func urgencyFor(eventType string) string {
	if eventType == "Warning" {
		return urgencyCritical
	}
	return urgencyNormal
}

// Package controller is the application-layer bridge between the EventSource
// input port and the Notifier output port. It observes events, buffers them for
// a configurable debounce window, maps each event onto a notification, and
// delivers the window — either as individual notifications or, above a
// configured threshold, as per-(Kind, Reason) summaries. It depends only on the
// two ports and references no concrete adapter, OS, or transport type.
package controller

import (
	"context"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	eventsPort "github.com/ebuildy/kubectl-notify/internal/port/datasources/events"
	notificationPort "github.com/ebuildy/kubectl-notify/internal/port/notification"
)

// Controller observes events and turns them into notifications. It buffers
// events received via OnEvent and flushes them on a fixed time window driven by
// Run, so a slow notifier never blocks the watch goroutine.
type Controller struct {
	notifier  notificationPort.Notifier
	logOut    io.Writer
	window    time.Duration
	threshold int

	mu     sync.Mutex
	buffer []eventsPort.Event
}

// compile-time guarantee that *Controller satisfies the Observer port.
var _ eventsPort.Observer = (*Controller)(nil)

// New constructs a Controller. notifier delivers notifications, logOut receives
// diagnostic lines (e.g. delivery failures), window is the debounce window (a
// window of zero flushes on every event), and threshold is the batch size above
// which a window is summarised instead of delivered individually. No global
// state is used; all dependencies are injected here.
func New(notifier notificationPort.Notifier, logOut io.Writer, window time.Duration, threshold int) *Controller {
	return &Controller{
		notifier:  notifier,
		logOut:    logOut,
		window:    window,
		threshold: threshold,
	}
}

// OnEvent buffers the event and returns promptly without delivering, so the
// watch goroutine is never blocked by notification delivery. When the window is
// zero, buffering is disabled and the event is flushed immediately.
func (c *Controller) OnEvent(ctx context.Context, e eventsPort.Event) error {
	c.mu.Lock()
	c.buffer = append(c.buffer, e)
	c.mu.Unlock()

	if c.window == 0 {
		c.flush(ctx)
	}
	return nil
}

// Run flushes buffered events once per window until ctx is cancelled, then
// performs a final flush of any remaining events and returns. When the window
// is zero there is nothing to tick on (OnEvent flushes inline); Run still waits
// for cancellation and performs a final flush.
func (c *Controller) Run(ctx context.Context) {
	if c.window <= 0 {
		<-ctx.Done()
		c.flush(context.Background())
		return
	}

	ticker := time.NewTicker(c.window)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.flush(ctx)
		case <-ctx.Done():
			// Final flush of anything still buffered. Use a fresh context so
			// delivery is not skipped just because the watch context is done.
			c.flush(context.Background())
			return
		}
	}
}

// flush swaps out the buffered events under the lock, then delivers them with
// the lock released so Notify is never called while the mutex is held.
func (c *Controller) flush(ctx context.Context) {
	c.mu.Lock()
	batch := c.buffer
	c.buffer = nil
	c.mu.Unlock()

	if len(batch) == 0 {
		return
	}

	if len(batch) <= c.threshold {
		for _, e := range batch {
			c.deliver(ctx, mapEvent(e))
		}
		return
	}

	for _, n := range summarise(batch) {
		c.deliver(ctx, n)
	}
}

// deliver sends one notification, logging and swallowing any error so a single
// delivery failure neither aborts the flush nor stops the watch.
func (c *Controller) deliver(ctx context.Context, n notificationPort.Notification) {
	if err := c.notifier.Notify(ctx, n); err != nil {
		_, _ = fmt.Fprintf(c.logOut, "controller: deliver notification %q: %v\n", n.Title, err)
	}
}

// mapEvent maps an event onto a notification: title from the object identity
// and reason, body from the message, and urgency from the event type. It
// degrades gracefully when identity fields are empty.
func mapEvent(e eventsPort.Event) notificationPort.Notification {
	return notificationPort.Notification{
		Title:   eventTitle(e),
		Body:    e.Message,
		Urgency: urgencyFor(e.Type),
	}
}

// eventTitle combines the object identity (Kind/Name) with the reason, omitting
// whichever parts are empty rather than producing stray separators.
func eventTitle(e eventsPort.Event) string {
	id := e.Kind
	if e.Name != "" {
		if id != "" {
			id += "/" + e.Name
		} else {
			id = e.Name
		}
	}

	switch {
	case id != "" && e.Reason != "":
		return id + ": " + e.Reason
	case id != "":
		return id
	default:
		return e.Reason
	}
}

// urgencyFor derives a notification urgency from an event type: "Warning" is
// critical, everything else (including "Normal" or unknown) is normal.
func urgencyFor(eventType string) notificationPort.Urgency {
	if eventType == "Warning" {
		return notificationPort.UrgencyCritical
	}
	return notificationPort.UrgencyNormal
}

// groupKey identifies a summary group: the cause of the events, i.e. the
// object kind and the event reason.
type groupKey struct {
	Kind   string
	Reason string
}

// summarise collapses a batch into one notification per distinct (Kind, Reason)
// group, with a body of the form "<count> events of <Kind>/<Reason>" and
// critical urgency when any event in the group is a Warning. Groups are emitted
// in a deterministic (sorted) order.
func summarise(batch []eventsPort.Event) []notificationPort.Notification {
	counts := make(map[groupKey]int)
	warning := make(map[groupKey]bool)
	order := make([]groupKey, 0)

	for _, e := range batch {
		k := groupKey{Kind: e.Kind, Reason: e.Reason}
		if _, seen := counts[k]; !seen {
			order = append(order, k)
		}
		counts[k]++
		if e.Type == "Warning" {
			warning[k] = true
		}
	}

	sort.Slice(order, func(i, j int) bool {
		if order[i].Kind != order[j].Kind {
			return order[i].Kind < order[j].Kind
		}
		return order[i].Reason < order[j].Reason
	})

	notifications := make([]notificationPort.Notification, 0, len(order))
	for _, k := range order {
		urgency := notificationPort.UrgencyNormal
		if warning[k] {
			urgency = notificationPort.UrgencyCritical
		}
		notifications = append(notifications, notificationPort.Notification{
			Title:   "kubectl-notify",
			Body:    fmt.Sprintf("%d events of %s/%s", counts[k], k.Kind, k.Reason),
			Urgency: urgency,
		})
	}
	return notifications
}

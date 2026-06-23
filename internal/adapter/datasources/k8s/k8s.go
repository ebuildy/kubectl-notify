// Package k8s implements the events.EventSource port by watching Kubernetes
// core/v1 Events via client-go and mapping each Kubernetes event onto the
// port's technology-agnostic events.Event value object.
package k8s

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	eventsPort "github.com/ebuildy/kubectl-notify/internal/port/datasources/events"
)

// Recognized filter keys. Any other key passed in the Filter is rejected so a
// misconfigured filter fails loudly rather than silently matching everything.
const (
	filterNamespace = "namespace"
	filterLabels    = "labels"
)

// Adapter watches Kubernetes core/v1 Events and forwards them as
// events.Event values to a registered observer.
type Adapter struct {
	client kubernetes.Interface
}

// compile-time guarantee that Adapter satisfies the EventSource port.
var _ eventsPort.EventSource = (*Adapter)(nil)

// New constructs a Kubernetes events adapter from a *rest.Config resolved from
// the standard kubectl connection flags (--kubeconfig, --context, --namespace).
func New(cfg *rest.Config) (*Adapter, error) {
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("k8s events: build client: %w", err)
	}
	return &Adapter{client: client}, nil
}

// NewWithClient constructs an adapter from an existing client, primarily for
// tests using a fake clientset.
func NewWithClient(client kubernetes.Interface) *Adapter {
	return &Adapter{client: client}
}

// Watch translates the Filter into Kubernetes watch options, then watches
// core/v1 Events (reconnecting on transient disconnects) and invokes the
// observer's OnEvent for each event in order. It blocks until ctx is cancelled
// or the observer returns an error, and returns a non-nil error wrapped with
// adapter context on stream failure.
func (a *Adapter) Watch(ctx context.Context, filter eventsPort.Filter, obs eventsPort.Observer) error {
	namespace, opts, err := buildWatchOptions(filter)
	if err != nil {
		return err
	}

	// Reconnect loop: a watch can end on a transient disconnect; resume from the
	// last seen resourceVersion until the caller cancels the context.
	for {
		if ctx.Err() != nil {
			return nil
		}

		w, err := a.client.CoreV1().Events(namespace).Watch(ctx, opts)
		if err != nil {
			// The context being cancelled is a clean shutdown, not a failure.
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("k8s events: start watch: %w", err)
		}

		resumeVersion, err := a.drain(ctx, w, obs)
		w.Stop()
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return nil
		}
		// Stream ended without error: reconnect from where we left off.
		opts.ResourceVersion = resumeVersion
	}
}

// drain consumes one watch stream, forwarding each Event to the observer until
// the stream closes or the context is cancelled. It returns the last observed
// resourceVersion so the caller can resume a reconnecting watch.
func (a *Adapter) drain(ctx context.Context, w watch.Interface, obs eventsPort.Observer) (resumeVersion string, err error) {
	for {
		select {
		case <-ctx.Done():
			return resumeVersion, nil
		case ev, ok := <-w.ResultChan():
			if !ok {
				return resumeVersion, nil
			}
			switch ev.Type {
			case watch.Error:
				return resumeVersion, fmt.Errorf("k8s events: watch error: %v", ev.Object)
			case watch.Added, watch.Modified:
				k8sEvent, ok := ev.Object.(*corev1.Event)
				if !ok {
					continue
				}
				resumeVersion = k8sEvent.ResourceVersion
				if err := obs.OnEvent(ctx, mapEvent(k8sEvent)); err != nil {
					return resumeVersion, fmt.Errorf("k8s events: observer: %w", err)
				}
			default:
				// Deleted/Bookmark: nothing to forward, but track the version.
				if obj, ok := ev.Object.(*corev1.Event); ok {
					resumeVersion = obj.ResourceVersion
				}
			}
		}
	}
}

// mapEvent converts a Kubernetes Event into the port's technology-agnostic
// Event, exposing no k8s.io types beyond this boundary.
func mapEvent(e *corev1.Event) eventsPort.Event {
	return eventsPort.Event{
		Reason:    e.Reason,
		Message:   e.Message,
		Type:      e.Type,
		Namespace: e.InvolvedObject.Namespace,
		Kind:      e.InvolvedObject.Kind,
		Name:      e.InvolvedObject.Name,
		Timestamp: eventTimestamp(e),
		Labels:    e.Labels,
	}
}

// eventTimestamp resolves the most meaningful occurrence time for a Kubernetes
// Event: the EventTime (used by the newer events API), falling back to the
// LastTimestamp, then the FirstTimestamp. Returns the zero time when none is
// set. No additional API calls are made.
func eventTimestamp(e *corev1.Event) time.Time {
	if !e.EventTime.IsZero() {
		return e.EventTime.Time
	}
	if !e.LastTimestamp.IsZero() {
		return e.LastTimestamp.Time
	}
	return e.FirstTimestamp.Time
}

// buildWatchOptions translates the generic Filter into a watched namespace and
// Kubernetes list options. An absent "namespace" key means all namespaces
// (metav1.NamespaceAll). Any unrecognized key returns an error naming the key.
func buildWatchOptions(filter eventsPort.Filter) (namespace string, opts metav1.ListOptions, err error) {
	namespace = metav1.NamespaceAll
	for key, value := range filter {
		switch key {
		case filterNamespace:
			namespace = value
		case filterLabels:
			opts.LabelSelector = value
		default:
			return "", metav1.ListOptions{}, fmt.Errorf("k8s events: unsupported filter key %q", key)
		}
	}
	return namespace, opts, nil
}

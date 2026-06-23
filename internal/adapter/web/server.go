// Package web implements the events.Observer port as a local web server: it
// keeps a bounded ring buffer of recent events, fans each new event out to
// connected browsers over WebSocket, and serves a self-contained embedded UI
// that renders the events as a vertical timeline.
package web

import (
	"context"
	"sync"

	eventsPort "github.com/ebuildy/kubectl-notify/internal/port/datasources/events"
)

// bufferCapacity is the maximum number of recent events retained in memory.
// When full, the oldest event is dropped.
const bufferCapacity = 100

// clientBuffer is the per-client channel depth. A client that falls this far
// behind has its new events dropped (it re-syncs from /api/events on
// reconnect) rather than blocking the watch goroutine.
const clientBuffer = 64

// Server implements events.Observer. OnEvent records each event in a bounded
// ring buffer and broadcasts it to connected WebSocket clients without ever
// blocking the watch goroutine.
type Server struct {
	mu      sync.Mutex
	buf     []eventsPort.Event
	clients map[chan eventDTO]struct{}
}

// compile-time guarantee that Server satisfies the Observer port.
var _ eventsPort.Observer = (*Server)(nil)

// NewServer constructs an empty web server.
func NewServer() *Server {
	return &Server{
		buf:     make([]eventsPort.Event, 0, bufferCapacity),
		clients: make(map[chan eventDTO]struct{}),
	}
}

// OnEvent appends the event to the ring buffer (dropping the oldest when full)
// and broadcasts it to every connected client with a non-blocking send. It
// returns promptly and never blocks on a slow client.
func (s *Server) OnEvent(_ context.Context, e eventsPort.Event) error {
	dto := toDTO(e)

	s.mu.Lock()
	if len(s.buf) >= bufferCapacity {
		// Drop the oldest to keep memory bounded.
		copy(s.buf, s.buf[1:])
		s.buf[len(s.buf)-1] = e
	} else {
		s.buf = append(s.buf, e)
	}
	// Snapshot the client channels so the broadcast runs without the lock.
	targets := make([]chan eventDTO, 0, len(s.clients))
	for ch := range s.clients {
		targets = append(targets, ch)
	}
	s.mu.Unlock()

	for _, ch := range targets {
		select {
		case ch <- dto:
		default:
			// Slow/full client: drop rather than block the watch goroutine.
		}
	}
	return nil
}

// snapshot returns a copy of the buffered events as DTOs in observation order
// (oldest first), for the /api/events initial render.
func (s *Server) snapshot() []eventDTO {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]eventDTO, len(s.buf))
	for i, e := range s.buf {
		out[i] = toDTO(e)
	}
	return out
}

// subscribe registers a new client channel and returns it along with an
// unsubscribe function that removes and drains it. Each subscriber receives
// every event observed after it subscribes.
func (s *Server) subscribe() (chan eventDTO, func()) {
	ch := make(chan eventDTO, clientBuffer)

	s.mu.Lock()
	s.clients[ch] = struct{}{}
	s.mu.Unlock()

	unsubscribe := func() {
		s.mu.Lock()
		delete(s.clients, ch)
		s.mu.Unlock()
	}
	return ch, unsubscribe
}

package web

import (
	"embed"
	"encoding/json"
	"net/http"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

//go:embed static/*
var staticFS embed.FS

// indexHTML is the single-page UI, loaded once from the embedded filesystem.
var indexHTML []byte

func init() {
	data, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		panic("web: embedded static/index.html missing: " + err.Error())
	}
	indexHTML = data
}

// Handler builds the HTTP router exposing the UI and the event API:
//   - GET /            → the embedded single-page UI
//   - GET /api/events  → JSON snapshot of buffered events (observation order)
//   - GET /ws          → WebSocket stream of each new event
//   - GET /healthz     → 200 ok
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/ws", s.handleWS)
	mux.Handle("/static/", http.FileServer(http.FS(staticFS)))
	mux.HandleFunc("/", s.handleIndex)
	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	// The catch-all also receives unknown paths; only the root serves the UI.
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(indexHTML)
}

func (s *Server) handleEvents(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(s.snapshot()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleWS upgrades to a WebSocket and writes one JSON event object per new
// event until the client disconnects or the request context is cancelled.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow() //nolint:errcheck // best-effort close on exit

	ctx := r.Context()

	ch, unsubscribe := s.subscribe()
	defer unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return
		case dto := <-ch:
			if err := wsjson.Write(ctx, conn, dto); err != nil {
				return
			}
		}
	}
}

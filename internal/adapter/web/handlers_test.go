package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	eventsPort "github.com/ebuildy/kubectl-notify/internal/port/datasources/events"
)

// TestHealthz asserts /healthz returns HTTP 200.
func TestHealthz(t *testing.T) {
	srv := httptest.NewServer(NewServer().Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

// TestEventsSnapshot asserts /api/events returns the buffered events as a JSON
// array in observation order with the derived urgency.
func TestEventsSnapshot(t *testing.T) {
	s := NewServer()
	_ = s.OnEvent(context.Background(), eventsPort.Event{Reason: "Started", Kind: "Pod", Name: "a", Type: "Normal"})
	_ = s.OnEvent(context.Background(), eventsPort.Event{Reason: "Failed", Kind: "Pod", Name: "b", Type: "Warning"})

	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/events")
	if err != nil {
		t.Fatalf("GET /api/events: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var got []eventDTO
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2", len(got))
	}
	if got[0].Name != "a" || got[1].Name != "b" {
		t.Errorf("events out of observation order: %+v", got)
	}
	if got[1].Urgency != urgencyCritical {
		t.Errorf("Warning urgency = %q, want %q", got[1].Urgency, urgencyCritical)
	}
}

// TestIndexServesEmbeddedPage asserts GET / returns the embedded HTML page.
func TestIndexServesEmbeddedPage(t *testing.T) {
	srv := httptest.NewServer(NewServer().Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body := make([]byte, 512)
	n, _ := resp.Body.Read(body)
	if !strings.Contains(string(body[:n]), "<!DOCTYPE html>") {
		t.Errorf("/ did not serve the embedded HTML page")
	}
}

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ebuildy/kubectl-notify/internal/app/daemon"
)

// liveState builds a State whose PID is this test process (always alive) so the
// liveness checks treat it as a running watcher.
func liveState() daemon.State {
	return daemon.State{
		PID:       os.Getpid(),
		StartTime: time.Now().Add(-2 * time.Minute),
		Namespace: "kube-system",
		Labels:    "app=nginx",
		Delay:     5 * time.Second,
		Max:       10,
		LogPath:   "/tmp/watch.log",
	}
}

// deadState builds a State whose PID is effectively never alive.
func deadState() daemon.State {
	st := liveState()
	st.PID = 1<<31 - 1
	return st
}

// TestEnsureNoLiveWatcher covers the single-instance guard used by the
// --background start path.
func TestEnsureNoLiveWatcher(t *testing.T) {
	t.Run("refuses when a live watcher exists", func(t *testing.T) {
		store := daemon.NewStore(filepath.Join(t.TempDir(), "watch.json"))
		if err := store.Write(liveState()); err != nil {
			t.Fatalf("Write: %v", err)
		}
		if err := ensureNoLiveWatcher(store); err == nil {
			t.Fatal("ensureNoLiveWatcher = nil, want refusal error for a live watcher")
		}
	})

	t.Run("allows when no record exists", func(t *testing.T) {
		store := daemon.NewStore(filepath.Join(t.TempDir(), "watch.json"))
		if err := ensureNoLiveWatcher(store); err != nil {
			t.Fatalf("ensureNoLiveWatcher = %v, want nil for absent record", err)
		}
	})

	t.Run("allows when the record is stale", func(t *testing.T) {
		store := daemon.NewStore(filepath.Join(t.TempDir(), "watch.json"))
		if err := store.Write(deadState()); err != nil {
			t.Fatalf("Write: %v", err)
		}
		if err := ensureNoLiveWatcher(store); err != nil {
			t.Fatalf("ensureNoLiveWatcher = %v, want nil for stale record", err)
		}
	})
}

// TestReportStatus covers the status rendering for running, missing, and stale
// state.
func TestReportStatus(t *testing.T) {
	t.Run("running prints detailed state", func(t *testing.T) {
		store := daemon.NewStore(filepath.Join(t.TempDir(), "watch.json"))
		st := liveState()
		if err := store.Write(st); err != nil {
			t.Fatalf("Write: %v", err)
		}

		var out bytes.Buffer
		now := st.StartTime.Add(2 * time.Minute)
		if err := reportStatus(&out, store, now); err != nil {
			t.Fatalf("reportStatus: %v", err)
		}

		got := out.String()
		for _, want := range []string{
			"background watch is running",
			"kube-system",
			"app=nginx",
			"5s",
			"max:       10",
			"2m0s",
			"/tmp/watch.log",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("status output missing %q\n--- output ---\n%s", want, got)
			}
		}
	})

	t.Run("missing prints not running", func(t *testing.T) {
		store := daemon.NewStore(filepath.Join(t.TempDir(), "watch.json"))
		var out bytes.Buffer
		if err := reportStatus(&out, store, time.Now()); err != nil {
			t.Fatalf("reportStatus: %v", err)
		}
		if !strings.Contains(out.String(), "no background watch is running") {
			t.Errorf("status output = %q, want not-running message", out.String())
		}
	})

	t.Run("stale prints not running and clears the file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "watch.json")
		store := daemon.NewStore(path)
		if err := store.Write(deadState()); err != nil {
			t.Fatalf("Write: %v", err)
		}

		var out bytes.Buffer
		if err := reportStatus(&out, store, time.Now()); err != nil {
			t.Fatalf("reportStatus: %v", err)
		}
		if !strings.Contains(out.String(), "no background watch is running") {
			t.Errorf("status output = %q, want not-running message", out.String())
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("stale state file not cleared (stat err = %v)", err)
		}
	})
}

// TestStopWatch covers the no-op stop paths (we avoid signalling a real foreign
// process in tests, so only the missing/stale paths are exercised here).
func TestStopWatch(t *testing.T) {
	t.Run("missing is a clean no-op", func(t *testing.T) {
		store := daemon.NewStore(filepath.Join(t.TempDir(), "watch.json"))
		var out bytes.Buffer
		if err := stopWatch(&out, store); err != nil {
			t.Fatalf("stopWatch: %v", err)
		}
		if !strings.Contains(out.String(), "no background watch is running") {
			t.Errorf("stop output = %q, want not-running message", out.String())
		}
	})

	t.Run("stale clears the file and reports not running", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "watch.json")
		store := daemon.NewStore(path)
		if err := store.Write(deadState()); err != nil {
			t.Fatalf("Write: %v", err)
		}

		var out bytes.Buffer
		if err := stopWatch(&out, store); err != nil {
			t.Fatalf("stopWatch: %v", err)
		}
		if !strings.Contains(out.String(), "no background watch is running") {
			t.Errorf("stop output = %q, want not-running message", out.String())
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("stale state file not cleared (stat err = %v)", err)
		}
	})
}

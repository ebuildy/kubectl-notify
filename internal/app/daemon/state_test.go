package daemon_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ebuildy/kubectl-notify/internal/app/daemon"
)

// TestStoreRoundTrip writes a State and reads it back unchanged, and verifies a
// missing file reads as not-found with no error.
func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "watch.json")
	store := daemon.NewStore(path)

	// A missing file is found=false, not an error.
	if _, found, err := store.Read(); err != nil || found {
		t.Fatalf("Read of missing file = (found=%v, err=%v), want (false, nil)", found, err)
	}

	want := daemon.State{
		PID:       4242,
		StartTime: time.Now().Truncate(time.Second).UTC(),
		Namespace: "kube-system",
		Labels:    "app=nginx",
		Delay:     5 * time.Second,
		Max:       10,
		LogPath:   "/tmp/watch.log",
	}
	if err := store.Write(want); err != nil {
		t.Fatalf("Write returned %v", err)
	}

	got, found, err := store.Read()
	if err != nil || !found {
		t.Fatalf("Read after Write = (found=%v, err=%v), want (true, nil)", found, err)
	}
	if got != want {
		t.Errorf("round-trip State = %+v, want %+v", got, want)
	}
}

// TestRemoveIdempotent verifies Remove on a missing file is a no-op and that a
// written file is gone after Remove.
func TestRemoveIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watch.json")
	store := daemon.NewStore(path)

	// Remove before any write: no error.
	if err := store.Remove(); err != nil {
		t.Fatalf("Remove of missing file returned %v, want nil", err)
	}

	if err := store.Write(daemon.State{PID: 1}); err != nil {
		t.Fatalf("Write returned %v", err)
	}
	if err := store.Remove(); err != nil {
		t.Fatalf("Remove returned %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("state file still present after Remove (stat err = %v)", err)
	}
}

// TestAliveAndUptime verifies Alive against the current process and a dead PID,
// and that Uptime is the difference between now and the start time.
func TestAliveAndUptime(t *testing.T) {
	if !daemon.Alive(os.Getpid()) {
		t.Errorf("Alive(self) = false, want true")
	}
	if daemon.Alive(0) {
		t.Errorf("Alive(0) = true, want false")
	}
	// PID 2^31-1 is effectively never a live process.
	if daemon.Alive(1<<31 - 1) {
		t.Errorf("Alive(huge pid) = true, want false")
	}

	start := time.Now().Add(-90 * time.Second)
	st := daemon.State{StartTime: start}
	now := start.Add(90 * time.Second)
	if got := st.Uptime(now); got != 90*time.Second {
		t.Errorf("Uptime = %v, want 90s", got)
	}
}

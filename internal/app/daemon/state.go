// Package daemon owns the on-disk state of the single background watcher: where
// the state file lives, reading/writing/removing it, and small helpers to tell
// whether a recorded process is still alive and how long it has been running.
// It holds no global state; callers inject a Store built from a resolved path.
package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// dirName is the per-user subdirectory holding kubectl-notify state.
const dirName = "kubectl-notify"

// stateFileName is the single state file tracking the background watcher.
const stateFileName = "watch.json"

// logFileName is the file the detached child's stdout/stderr is redirected to.
const logFileName = "watch.log"

// State is the persisted description of the background watcher. All fields are
// primitive so the file stays portable and human-readable.
type State struct {
	PID       int           `json:"pid"`
	StartTime time.Time     `json:"startTime"`
	Namespace string        `json:"namespace"`
	Labels    string        `json:"labels"`
	Delay     time.Duration `json:"delay"`
	Max       int           `json:"max"`
	LogPath   string        `json:"logPath"`
}

// Uptime reports how long the watcher has been running relative to now.
func (s State) Uptime(now time.Time) time.Duration {
	return now.Sub(s.StartTime)
}

// Alive reports whether a process with the given PID currently exists. It sends
// the no-op signal 0, which performs the kernel's existence/permission check
// without delivering a signal. A non-positive PID is never alive.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// Terminate sends SIGTERM to the process with the given PID so its own signal
// handler can shut down cleanly (performing any final flush).
func Terminate(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("daemon: find process %d: %w", pid, err)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("daemon: signal process %d: %w", pid, err)
	}
	return nil
}

// stateDir resolves the per-user directory holding kubectl-notify state.
func stateDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("daemon: resolve cache dir: %w", err)
	}
	return filepath.Join(base, dirName), nil
}

// DefaultStatePath returns the default path of the watcher state file under the
// user's cache directory.
func DefaultStatePath() (string, error) {
	dir, err := stateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, stateFileName), nil
}

// DefaultLogPath returns the default path of the detached child's log file
// under the user's cache directory.
func DefaultLogPath() (string, error) {
	dir, err := stateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, logFileName), nil
}

// Store reads and writes the watcher State at a fixed file path. The path is
// injected so tests can point at a temporary directory.
type Store struct {
	path string
}

// NewStore constructs a Store backed by the given file path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Path reports the file path this Store manages.
func (s *Store) Path() string { return s.path }

// Read loads the persisted State. The boolean is false (with a nil error) when
// no state file exists yet.
func (s *Store) Read() (State, bool, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return State{}, false, nil
	}
	if err != nil {
		return State{}, false, fmt.Errorf("daemon: read state: %w", err)
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return State{}, false, fmt.Errorf("daemon: decode state: %w", err)
	}
	return st, true, nil
}

// Write persists the State, creating the containing directory as needed. It
// writes to a temporary file and renames it into place so a reader never sees a
// half-written file.
func (s *Store) Write(st State) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("daemon: create state dir: %w", err)
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("daemon: encode state: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("daemon: write state: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("daemon: replace state: %w", err)
	}
	return nil
}

// Remove deletes the state file. It is idempotent: a missing file is not an
// error.
func (s *Store) Remove() error {
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("daemon: remove state: %w", err)
	}
	return nil
}

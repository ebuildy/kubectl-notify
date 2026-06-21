package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// HiddenStateFlag is the name of the hidden flag passed to the detached child
// so it knows which state file it owns and must remove on exit. It is hidden
// from --help; operators never set it directly.
const HiddenStateFlag = "__daemon-statefile"

// SpawnConfig describes the detached background watch to launch.
type SpawnConfig struct {
	Namespace string
	Labels    string
	Delay     time.Duration
	Max       int
	StatePath string
	LogPath   string
}

// Spawn re-executes this binary as a detached `watch` child process running the
// same parameters (without --background), with stdout/stderr redirected to the
// log file and the child placed in its own session/process group so it survives
// the parent terminal. It returns the child PID and does not wait on it.
func Spawn(cfg SpawnConfig) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("daemon: resolve executable: %w", err)
	}

	logFile, err := openLog(cfg.LogPath)
	if err != nil {
		return 0, err
	}
	// The child inherits its own copy of the file descriptor; close ours once
	// the process has been started.
	defer func() { _ = logFile.Close() }()

	args := []string{
		"watch",
		"--delay", cfg.Delay.String(),
		"--max", strconv.Itoa(cfg.Max),
		"--" + HiddenStateFlag, cfg.StatePath,
	}
	if cfg.Namespace != "" {
		args = append(args, "--namespace", cfg.Namespace)
	}
	if cfg.Labels != "" {
		args = append(args, "--labels", cfg.Labels)
	}

	cmd := exec.Command(exe, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = detachSysProcAttr()

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("daemon: start detached watch: %w", err)
	}
	pid := cmd.Process.Pid
	// Release the child so the parent carries no wait obligation; the detached
	// process keeps running independently.
	_ = cmd.Process.Release()
	return pid, nil
}

// openLog creates the log directory if needed and opens the log file for
// appending.
func openLog(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("daemon: create log dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("daemon: open log file: %w", err)
	}
	return f, nil
}

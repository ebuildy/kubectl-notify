package cmd

import (
	"testing"

	"k8s.io/cli-runtime/pkg/genericiooptions"
)

// TestNewWebCommandWiring asserts the web command is constructed with the
// expected name and flags, mirroring the watch command smoke test.
func TestNewWebCommandWiring(t *testing.T) {
	cmd := newWebCommand(genericiooptions.IOStreams{})

	if cmd.Use != "web" {
		t.Errorf("Use = %q, want web", cmd.Use)
	}
	if cmd.RunE == nil {
		t.Error("RunE is nil; command is not runnable")
	}

	for _, name := range []string{"labels", "port", "no-open"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("missing --%s flag", name)
		}
	}

	if got := cmd.Flags().Lookup("port").DefValue; got != "0" {
		t.Errorf("--port default = %q, want 0", got)
	}
	if got := cmd.Flags().Lookup("no-open").DefValue; got != "false" {
		t.Errorf("--no-open default = %q, want false", got)
	}
}

// TestWebCommandRegistered asserts the web command is wired into the root.
func TestWebCommandRegistered(t *testing.T) {
	root := NewRootCommand(genericiooptions.IOStreams{})
	for _, c := range root.Commands() {
		if c.Name() == "web" {
			return
		}
	}
	t.Error("web command not registered on root")
}

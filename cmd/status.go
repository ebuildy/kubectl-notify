package cmd

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/ebuildy/kubectl-notify/internal/app/daemon"
)

// newStatusCommand builds the `kubectl notify status` subcommand, which reports
// the state of the background watcher without contacting the cluster.
func newStatusCommand(streams genericiooptions.IOStreams) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Report the state of the background watch",
		Long: `status reports whether a background 'kubectl notify watch --background' is
running and, if so, its PID, the namespace/labels filter, the debounce window
(--delay), the batch threshold (--max), how long it has been running, and the
log file path. It does not contact the cluster.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			statePath, err := daemon.DefaultStatePath()
			if err != nil {
				_, _ = fmt.Fprintln(streams.ErrOut, "Error:", err)
				return err
			}
			if err := reportStatus(streams.Out, daemon.NewStore(statePath), time.Now()); err != nil {
				_, _ = fmt.Fprintln(streams.ErrOut, "Error:", err)
				return err
			}
			return nil
		},
	}
}

// reportStatus prints the watcher state from the store. A missing record, or a
// stale record whose process is gone, prints a "not running" line and clears
// the stale file. Either way it returns nil (not running is not an error).
func reportStatus(out io.Writer, store *daemon.Store, now time.Time) error {
	st, found, err := store.Read()
	if err != nil {
		return err
	}
	if !found || !daemon.Alive(st.PID) {
		if found {
			if err := store.Remove(); err != nil {
				return err
			}
		}
		_, _ = fmt.Fprintln(out, "no background watch is running")
		return nil
	}

	namespace := st.Namespace
	if namespace == "" {
		namespace = "all namespaces"
	}
	labels := st.Labels
	if labels == "" {
		labels = "none"
	}

	_, _ = fmt.Fprintln(out, "background watch is running")
	_, _ = fmt.Fprintf(out, "  PID:       %d\n", st.PID)
	_, _ = fmt.Fprintf(out, "  namespace: %s\n", namespace)
	_, _ = fmt.Fprintf(out, "  labels:    %s\n", labels)
	_, _ = fmt.Fprintf(out, "  delay:     %s\n", st.Delay)
	_, _ = fmt.Fprintf(out, "  max:       %d\n", st.Max)
	_, _ = fmt.Fprintf(out, "  uptime:    %s\n", st.Uptime(now).Round(time.Second))
	_, _ = fmt.Fprintf(out, "  log:       %s\n", st.LogPath)
	return nil
}

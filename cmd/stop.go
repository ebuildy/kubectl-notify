package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/ebuildy/kubectl-notify/internal/app/daemon"
)

// newStopCommand builds the `kubectl notify stop` subcommand, which stops the
// background watcher (if any) with a clean termination signal.
func newStopCommand(streams genericiooptions.IOStreams) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the background watch",
		Long: `stop terminates a background 'kubectl notify watch --background' by sending it
SIGTERM, so it performs its clean final flush, and then clears the recorded
state. When no background watch is running it reports that and exits 0.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			statePath, err := daemon.DefaultStatePath()
			if err != nil {
				_, _ = fmt.Fprintln(streams.ErrOut, "Error:", err)
				return err
			}
			if err := stopWatch(streams.Out, daemon.NewStore(statePath)); err != nil {
				_, _ = fmt.Fprintln(streams.ErrOut, "Error:", err)
				return err
			}
			return nil
		},
	}
}

// stopWatch signals the recorded watcher to terminate and clears its state. A
// missing record, or a stale record whose process is gone, is a clean no-op
// that clears any stale file and returns nil.
func stopWatch(out io.Writer, store *daemon.Store) error {
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

	if err := daemon.Terminate(st.PID); err != nil {
		return err
	}
	if err := store.Remove(); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "stopped background watch (PID %d)\n", st.PID)
	return nil
}

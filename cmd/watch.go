package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/ebuildy/kubectl-notify/internal/adapter/datasources/k8s"
	"github.com/ebuildy/kubectl-notify/internal/adapter/notification/desktop"
	"github.com/ebuildy/kubectl-notify/internal/app/controller"
	"github.com/ebuildy/kubectl-notify/internal/app/daemon"
	eventsPort "github.com/ebuildy/kubectl-notify/internal/port/datasources/events"
)

// buildFilter translates the resolved namespace and labels into the generic
// events.Filter. The namespace key is set only when a namespace is given, and
// the labels key only when labels are given; an empty filter watches all
// namespaces with no label selector.
func buildFilter(namespace, labels string) eventsPort.Filter {
	filter := eventsPort.Filter{}
	if namespace != "" {
		filter["namespace"] = namespace
	}
	if labels != "" {
		filter["labels"] = labels
	}
	return filter
}

// ensureNoLiveWatcher returns an error when a background watcher is already
// recorded and its process is still alive, so a second one is refused. An
// absent record, or a stale record whose process is gone, returns nil (the
// caller is clear to start).
func ensureNoLiveWatcher(store *daemon.Store) error {
	st, found, err := store.Read()
	if err != nil {
		return err
	}
	if found && daemon.Alive(st.PID) {
		return fmt.Errorf("a background watch is already running (PID %d); run `kubectl notify stop` first", st.PID)
	}
	return nil
}

// newWatchCommand builds the `kubectl notify watch` subcommand, which composes
// the Kubernetes events adapter, the controller, and the desktop adapter into a
// running pipeline that turns cluster events into desktop notifications until
// the user interrupts it. With --background it instead detaches a child process
// running this same pipeline and returns immediately.
func newWatchCommand(streams genericiooptions.IOStreams) *cobra.Command {
	var (
		labels     string
		delay      time.Duration
		maxEvents  int
		background bool
		stateFile  string
	)

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch Kubernetes events and surface them as desktop notifications",
		Long: `watch streams Kubernetes events through the controller and the desktop
notifier, turning each event into a notification. Events are buffered for a
debounce window (--delay) and, when more than --max arrive in a window, are
collapsed into per-kind/reason summaries instead of one toast per event. It
honors the standard kubectl connection flags and runs until interrupted
(Ctrl-C / SIGINT).

With --background the watch is launched as a detached background process and the
command returns immediately; use 'kubectl notify status' and 'kubectl notify
stop' to inspect and stop it.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			namespace := ""
			if configFlags.Namespace != nil {
				namespace = *configFlags.Namespace
			}

			if background {
				return startBackgroundWatch(streams, namespace, labels, delay, maxEvents)
			}

			// Child path: the detached child owns the state file named here and
			// removes it on a clean exit so status/stop report nothing running.
			if stateFile != "" {
				store := daemon.NewStore(stateFile)
				defer func() { _ = store.Remove() }()
			}

			return runForegroundWatch(cmd.Context(), streams, namespace, labels, delay, maxEvents)
		},
	}

	cmd.Flags().StringVar(&labels, "labels", "", "label selector to filter events (e.g. app=nginx)")
	cmd.Flags().DurationVar(&delay, "delay", 5*time.Second, "debounce window: buffer events for this long before notifying")
	cmd.Flags().IntVar(&maxEvents, "max", 10, "batch threshold: above this many events per window, deliver per-kind/reason summaries")
	cmd.Flags().BoolVar(&background, "background", false, "run the watch as a detached background process and return immediately")

	// Internal flag used by --background to tell the detached child which state
	// file it owns; hidden from --help.
	cmd.Flags().StringVar(&stateFile, daemon.HiddenStateFlag, "", "internal: state file owned by the detached child")
	_ = cmd.Flags().MarkHidden(daemon.HiddenStateFlag)

	return cmd
}

// startBackgroundWatch enforces the single-watcher rule, spawns a detached child
// running the same watch, records its state, and reports where it is running.
func startBackgroundWatch(streams genericiooptions.IOStreams, namespace, labels string, delay time.Duration, maxEvents int) error {
	statePath, err := daemon.DefaultStatePath()
	if err != nil {
		_, _ = fmt.Fprintln(streams.ErrOut, "Error:", err)
		return err
	}
	logPath, err := daemon.DefaultLogPath()
	if err != nil {
		_, _ = fmt.Fprintln(streams.ErrOut, "Error:", err)
		return err
	}

	store := daemon.NewStore(statePath)
	if err := ensureNoLiveWatcher(store); err != nil {
		_, _ = fmt.Fprintln(streams.ErrOut, "Error:", err)
		return err
	}

	pid, err := daemon.Spawn(daemon.SpawnConfig{
		Namespace: namespace,
		Labels:    labels,
		Delay:     delay,
		Max:       maxEvents,
		StatePath: statePath,
		LogPath:   logPath,
	})
	if err != nil {
		_, _ = fmt.Fprintln(streams.ErrOut, "Error:", err)
		return err
	}

	st := daemon.State{
		PID:       pid,
		StartTime: time.Now(),
		Namespace: namespace,
		Labels:    labels,
		Delay:     delay,
		Max:       maxEvents,
		LogPath:   logPath,
	}
	if err := store.Write(st); err != nil {
		_, _ = fmt.Fprintln(streams.ErrOut, "Error:", err)
		return err
	}

	_, _ = fmt.Fprintf(streams.Out, "background watch started (PID %d)\nlogs: %s\n", pid, logPath)
	return nil
}

// runForegroundWatch runs the event-to-notification pipeline in the foreground,
// blocking until ctx is cancelled or an interrupt is received.
func runForegroundWatch(ctx context.Context, streams genericiooptions.IOStreams, namespace, labels string, delay time.Duration, maxEvents int) error {
	cfg, err := configFlags.ToRESTConfig()
	if err != nil {
		_, _ = fmt.Fprintln(streams.ErrOut, "Error:", err)
		return err
	}

	source, err := k8s.New(cfg)
	if err != nil {
		_, _ = fmt.Fprintln(streams.ErrOut, "Error:", err)
		return err
	}

	notifier := desktop.New()
	ctrl := controller.New(notifier, streams.ErrOut, delay, maxEvents)

	filter := buildFilter(namespace, labels)

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	go ctrl.Run(ctx)

	if err := source.Watch(ctx, filter, ctrl); err != nil {
		_, _ = fmt.Fprintln(streams.ErrOut, "Error:", err)
		return err
	}
	return nil
}

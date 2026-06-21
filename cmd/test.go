package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/ebuildy/kubectl-notify/internal/adapter/notification/desktop"
	notificationPort "github.com/ebuildy/kubectl-notify/internal/port/notification"
)

// levelToUrgency maps the --level flag value to a port urgency. The boolean
// reports whether the value was recognised.
func levelToUrgency(level string) (notificationPort.Urgency, bool) {
	switch level {
	case "low":
		return notificationPort.UrgencyLow, true
	case "normal":
		return notificationPort.UrgencyNormal, true
	case "critical":
		return notificationPort.UrgencyCritical, true
	default:
		return 0, false
	}
}

// newTestCommand builds the `kubectl notify test` subcommand, which sends a
// sample notification through the Notifier port using the desktop adapter.
func newTestCommand(streams genericiooptions.IOStreams) *cobra.Command {
	var (
		title string
		body  string
		level string
	)

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Send a sample desktop notification to exercise the notifier",
		Long: `test sends a single sample notification through the Notifier port using the
desktop adapter, so the output side of the plugin can be exercised without any
Kubernetes connection.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			urgency, ok := levelToUrgency(level)
			if !ok {
				err := fmt.Errorf("invalid --level %q: accepted levels are low, normal, critical", level)
				_, _ = fmt.Fprintln(streams.ErrOut, "Error:", err)
				return err
			}

			notifier := desktop.New()

			err := notifier.Notify(cmd.Context(), notificationPort.Notification{
				Title:   title,
				Body:    body,
				Urgency: urgency,
			})
			if err != nil {
				_, _ = fmt.Fprintln(streams.ErrOut, "Error:", err)
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "kubectl-notify", "notification title")
	cmd.Flags().StringVar(&body, "body", "This is a test notification.", "notification body")
	cmd.Flags().StringVar(&level, "level", "normal", "urgency level: low, normal, or critical")

	return cmd
}

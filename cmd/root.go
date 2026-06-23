package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
)

// version is set at build time via -ldflags.
var version = "dev"

// configFlags holds the standard kubectl connection flags
// (--kubeconfig, --context, --namespace, --token, ...).
var configFlags = genericclioptions.NewConfigFlags(true)

// NewRootCommand builds the root `kubectl notify` cobra command.
func NewRootCommand(streams genericiooptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubectl-notify",
		Short: "Surface Kubernetes and FluxCD events as notifications",
		Long: `kubectl-notify is a kubectl plugin that watches events from Kubernetes
or FluxCD and turns them into desktop notifications, a local web UI, and more.`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	configFlags.AddFlags(cmd.PersistentFlags())

	cmd.AddCommand(newTestCommand(streams))
	cmd.AddCommand(newWatchCommand(streams))
	cmd.AddCommand(newWebCommand(streams))
	cmd.AddCommand(newStatusCommand(streams))
	cmd.AddCommand(newStopCommand(streams))

	return cmd
}

// Execute runs the root command against the process std streams.
func Execute() error {
	streams := genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	return NewRootCommand(streams).Execute()
}

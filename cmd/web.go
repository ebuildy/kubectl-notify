package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/ebuildy/kubectl-notify/internal/adapter/datasources/k8s"
	"github.com/ebuildy/kubectl-notify/internal/adapter/web"
)

// newWebCommand builds the `kubectl notify web` subcommand: it starts a local
// HTTP server, opens the default browser at its URL, and streams cluster events
// to the page as a live timeline until interrupted (Ctrl-C / SIGTERM).
func newWebCommand(streams genericiooptions.IOStreams) *cobra.Command {
	var (
		labels string
		port   int
		noOpen bool
	)

	cmd := &cobra.Command{
		Use:   "web",
		Short: "Serve a local web UI streaming Kubernetes events as a live timeline",
		Long: `web starts a local HTTP server, opens your default browser at its URL, and
streams Kubernetes events to the page as a vertical timeline (newest on top,
grouped into columns by reason and kind). It honors the standard kubectl
connection flags and the --labels selector, binds to loopback only, performs
read-only operations, and runs until interrupted (Ctrl-C / SIGINT).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			namespace := ""
			if configFlags.Namespace != nil {
				namespace = *configFlags.Namespace
			}
			return runWeb(cmd.Context(), streams, namespace, labels, port, noOpen)
		},
	}

	cmd.Flags().StringVar(&labels, "labels", "", "label selector to filter events (e.g. app=nginx)")
	cmd.Flags().IntVar(&port, "port", 0, "local port to bind (0 = OS-chosen ephemeral port)")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "do not open the browser; only print the URL")

	return cmd
}

// runWeb wires the Kubernetes events source to the web server, binds the HTTP
// listener on loopback, prints (and optionally opens) the URL, and runs the
// server and the watch concurrently until the context is cancelled.
func runWeb(ctx context.Context, streams genericiooptions.IOStreams, namespace, labels string, port int, noOpen bool) error {
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

	server := web.NewServer()

	// Bind first so we can report the real URL (including an OS-chosen port).
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		_, _ = fmt.Fprintln(streams.ErrOut, "Error:", err)
		return err
	}

	url := fmt.Sprintf("http://%s", ln.Addr().String())
	_, _ = fmt.Fprintf(streams.Out, "kubectl-notify web UI: %s\n", url)

	if !noOpen {
		if err := web.OpenBrowser(url); err != nil {
			_, _ = fmt.Fprintln(streams.ErrOut, "warning: could not open browser:", err)
		}
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpServer := &http.Server{
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Run the HTTP server and the watch concurrently; the first to fail (or the
	// context cancellation) tears both down cleanly.
	errCh := make(chan error, 2)

	go func() {
		if err := httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("web: http server: %w", err)
			return
		}
		errCh <- nil
	}()

	go func() {
		errCh <- source.Watch(ctx, buildFilter(namespace, labels), server)
	}()

	// Wait for cancellation or the first error from either goroutine.
	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-errCh:
	}

	// Cancel the watch and shut the HTTP server down with a bounded grace period.
	stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)

	// Drain the remaining goroutine so neither is leaked.
	<-errCh

	if runErr != nil {
		_, _ = fmt.Fprintln(streams.ErrOut, "Error:", runErr)
		return runErr
	}
	return nil
}

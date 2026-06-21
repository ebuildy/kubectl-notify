# watch-command Specification

## Purpose
TBD - created by archiving change add-notification-controller. Update Purpose after archive.
## Requirements
### Requirement: Watch command runs the event-to-notification pipeline

The system SHALL provide a `kubectl notify watch` subcommand that composes the
Kubernetes events adapter, the event-notification controller, and the desktop
notification adapter into a running pipeline. The command SHALL honor the standard
kubectl connection flags (`--kubeconfig`, `--context`, `--namespace`) when building
the Kubernetes adapter, and SHALL run until the underlying event stream ends or the
process receives an interrupt signal. On a clean shutdown the command SHALL exit
with code `0`.

#### Scenario: Watch streams events as notifications

- **WHEN** a user runs `kubectl notify watch` against a reachable cluster and a matching event occurs
- **THEN** the event is delivered as a desktop notification through the controller and the notifier port

#### Scenario: Interrupt stops the watch cleanly

- **WHEN** the user interrupts the running `kubectl notify watch` command (SIGINT)
- **THEN** the watch stops, the command returns without error, and the process exits with code `0`

#### Scenario: Connection failure is reported

- **WHEN** `kubectl notify watch` cannot establish the watch (for example, the cluster is unreachable)
- **THEN** the command prints the error to stderr and exits with a non-zero status

### Requirement: Watch command builds the filter from flags

The `kubectl notify watch` command SHALL translate its flags into the generic
`events.Filter` passed to the `EventSource`: the resolved namespace (from
`--namespace`) SHALL populate the filter's `namespace` key when a namespace is set,
and a `--labels` flag SHALL populate the filter's `labels` key when provided. When
no namespace is set, the command SHALL watch across all namespaces.

#### Scenario: Namespace flag scopes the watch

- **WHEN** a user runs `kubectl notify watch --namespace kube-system`
- **THEN** the command passes a filter with the `namespace` key set to `kube-system` to the event source

#### Scenario: Labels flag scopes the watch

- **WHEN** a user runs `kubectl notify watch --labels app=nginx`
- **THEN** the command passes a filter with the `labels` key set to `app=nginx` to the event source

#### Scenario: No scoping flags watches all namespaces

- **WHEN** a user runs `kubectl notify watch` with no namespace and no labels
- **THEN** the command passes an empty filter and the watch spans all namespaces

### Requirement: Watch command exposes debounce and batch flags

The `kubectl notify watch` command SHALL expose a `--delay` flag (a duration) that
sets the controller's debounce window and a `--max` flag (an integer) that sets the
batch threshold, and SHALL pass both into the controller. Both flags SHALL have
sensible defaults so the command works without specifying them.

#### Scenario: Delay flag sets the debounce window

- **WHEN** a user runs `kubectl notify watch --delay 10s`
- **THEN** the controller buffers events for ten seconds before flushing

#### Scenario: Max flag sets the batch threshold

- **WHEN** a user runs `kubectl notify watch --max 3` and more than three events arrive in a window
- **THEN** the window is delivered as per-kind/reason summary notifications rather than individual ones

#### Scenario: Defaults apply when flags are omitted

- **WHEN** a user runs `kubectl notify watch` without `--delay` or `--max`
- **THEN** the command applies its default window and threshold to the controller


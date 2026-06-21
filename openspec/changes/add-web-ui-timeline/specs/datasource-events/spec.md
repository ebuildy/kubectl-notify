## MODIFIED Requirements

### Requirement: Event value object

The system SHALL define an `Event` value object carried through the `EventSource`
port as a struct so that call sites stay stable as fields are added. The `Event`
MUST capture, at minimum, a reason, a message, a type (for example normal or
warning), the identity of the object the event refers to (namespace, kind,
name), a timestamp indicating when the event occurred, and an optional set of
labels associated with the event, using only technology-agnostic field types. The
timestamp and labels fields are additive; existing consumers that do not read them
MUST be unaffected.

#### Scenario: Event carries source-agnostic fields

- **WHEN** an adapter constructs an `Event`
- **THEN** it can set a reason, a message, a type, the referenced object's namespace, kind, and name, a timestamp, and a set of labels without exposing any Kubernetes or transport types

#### Scenario: Timestamp and labels are optional

- **WHEN** an adapter constructs an `Event` without setting a timestamp or labels
- **THEN** the `Event` is still valid and existing consumers behave exactly as before

### Requirement: Kubernetes events adapter

The system SHALL provide an adapter that implements the `EventSource` port by
watching Kubernetes `core/v1` Events via `client-go` and mapping each Kubernetes
event onto the port's `Event` value object. The adapter MUST honor the standard
kubectl connection configuration (`--kubeconfig`, `--context`, `--namespace`) when
establishing the watch, MUST translate the generic `Filter` into Kubernetes watch
options (at least `namespace` into the watched namespace and `labels` into a label
selector), MUST translate context cancellation into a clean shutdown of the
watch, and MUST populate the `Event` timestamp (from the Kubernetes event's event
time, falling back to its last or first timestamp) and the `Event` labels (from the
Kubernetes event object's own metadata labels) without making additional API calls.

#### Scenario: Kubernetes events are forwarded

- **WHEN** the Kubernetes adapter is watching and a new Event matching the filter appears in the cluster
- **THEN** the adapter maps it to an `Event` and invokes the observer's `OnEvent` with that value

#### Scenario: Filter is translated to Kubernetes watch options

- **WHEN** the adapter receives a `Filter` with a `namespace` and/or `labels` key
- **THEN** it watches Events only in that namespace and/or matching that label selector; when no `namespace` is set it watches across all namespaces

#### Scenario: Context cancellation stops the watch

- **WHEN** the context passed to the adapter is cancelled
- **THEN** the adapter stops the Kubernetes watch and the blocking call returns without leaking goroutines

#### Scenario: Timestamp and labels are mapped

- **WHEN** the adapter maps a Kubernetes Event that has an event time and metadata labels
- **THEN** the resulting `Event` carries a timestamp derived from the Kubernetes event time (or its last/first timestamp when event time is absent) and the event object's metadata labels

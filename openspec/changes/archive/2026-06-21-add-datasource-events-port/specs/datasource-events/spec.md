## ADDED Requirements

### Requirement: EventSource port

The system SHALL define a technology-agnostic `EventSource` port (interface) that
watches a source of events, subject to a `Filter`, and forwards each matching event
to a registered `Observer`, following the Observer pattern (see ADR-001). The system
SHALL define an `Observer` interface with an `OnEvent` method, and an `ObserverFunc`
adapter so a plain function can satisfy `Observer`. The `Watch` method SHALL accept a
`Filter`. The port MUST NOT reference any concrete event technology (no Kubernetes,
OS, library, or transport details). The watch operation SHALL block the calling
goroutine until the provided context is cancelled or the underlying stream ends, and
SHALL return an error describing the adapter when the stream fails.

#### Scenario: Watch notifies the observer for each matching event

- **WHEN** a caller starts the `EventSource` with a `Filter` and a registered `Observer`
- **THEN** the observer's `OnEvent` is invoked once for every event matching the filter that the source produces, in the order received

#### Scenario: Watch blocks until context cancellation

- **WHEN** a caller starts the `EventSource` with a context
- **THEN** the call blocks and returns nil (or the stream's terminal error) only after the context is cancelled or the stream ends

#### Scenario: Stream failure surfaces an error

- **WHEN** the underlying source fails to establish or maintain the watch
- **THEN** the `EventSource` returns a non-nil error wrapped with context describing the adapter that failed

### Requirement: Event value object

The system SHALL define an `Event` value object carried through the `EventSource`
port as a struct so that call sites stay stable as fields are added. The `Event`
MUST capture, at minimum, a reason, a message, a type (for example normal or
warning), and the identity of the object the event refers to (namespace, kind,
name), using only technology-agnostic field types.

#### Scenario: Event carries source-agnostic fields

- **WHEN** an adapter constructs an `Event`
- **THEN** it can set a reason, a message, a type, and the referenced object's namespace, kind, and name without exposing any Kubernetes or transport types

### Requirement: Filter value object

The system SHALL define a generic, technology-agnostic `Filter` as a
`map[string]string` of filter keys to values. The `Filter` MUST NOT reference any
concrete event technology. Each adapter SHALL translate the recognized keys into its
own native filtering mechanism, and SHALL return an error identifying any key it does
not support, so that a misconfigured filter fails loudly rather than silently
matching everything. An empty or nil `Filter` SHALL mean "no filtering".

#### Scenario: Filter carries generic key/value pairs

- **WHEN** a caller constructs a `Filter`
- **THEN** it can set arbitrary string keys to string values without referencing any adapter-specific type

#### Scenario: Unsupported filter key fails loudly

- **WHEN** an adapter receives a `Filter` containing a key it does not support
- **THEN** `Watch` returns a non-nil error naming the unsupported key, and does not start the watch

#### Scenario: Empty filter means no filtering

- **WHEN** a caller passes a nil or empty `Filter`
- **THEN** the adapter watches without applying any filter

### Requirement: Kubernetes events adapter

The system SHALL provide an adapter that implements the `EventSource` port by
watching Kubernetes `core/v1` Events via `client-go` and mapping each Kubernetes
event onto the port's `Event` value object. The adapter MUST honor the standard
kubectl connection configuration (`--kubeconfig`, `--context`, `--namespace`) when
establishing the watch, MUST translate the generic `Filter` into Kubernetes watch
options (at least `namespace` into the watched namespace and `labels` into a label
selector), and MUST translate context cancellation into a clean shutdown of the
watch.

#### Scenario: Kubernetes events are forwarded

- **WHEN** the Kubernetes adapter is watching and a new Event matching the filter appears in the cluster
- **THEN** the adapter maps it to an `Event` and invokes the observer's `OnEvent` with that value

#### Scenario: Filter is translated to Kubernetes watch options

- **WHEN** the adapter receives a `Filter` with a `namespace` and/or `labels` key
- **THEN** it watches Events only in that namespace and/or matching that label selector; when no `namespace` is set it watches across all namespaces

#### Scenario: Context cancellation stops the watch

- **WHEN** the context passed to the adapter is cancelled
- **THEN** the adapter stops the Kubernetes watch and the blocking call returns without leaking goroutines

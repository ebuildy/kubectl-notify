## 1. EventSource port

- [x] 1.1 Create `internal/port/datasources/events/events.go` with the `Event` value
  object (`Reason`, `Message`, `Type`, and object ref `Namespace`/`Kind`/`Name`),
  package doc explaining it is technology-agnostic.
- [x] 1.2 Define the Observer pattern per ADR-001: an `Observer` interface
  (`OnEvent(ctx, Event) error`), an `ObserverFunc` adapter, and the `EventSource`
  interface (`Watch(ctx context.Context, filter Filter, obs Observer) error`),
  documenting the blocking semantics and context-cancellation contract.
- [x] 1.3 Define `type Filter map[string]string` with a doc comment stating it is a
  generic, technology-agnostic filter that each adapter translates; nil/empty means
  no filtering.
- [x] 1.4 Add a white-box unit test for the port using a stub source that verifies
  the observer's `OnEvent` is invoked per matching event and that cancelling the
  context returns.

## 2. Kubernetes events adapter

- [x] 2.1 Create `internal/adapter/datasources/k8s/k8s.go` with an `Adapter` struct,
  a `New(...)` constructor taking a `*rest.Config` (resolved from kubectl flags), and
  a compile-time `var _ eventsPort.EventSource = (*Adapter)(nil)` assertion (import
  alias `eventsPort`).
- [x] 2.2 Translate the `Filter` into Kubernetes watch options: `namespace` → watched
  namespace (absent ⇒ all namespaces), `labels` → label selector; return an error
  naming any unsupported key before starting the watch.
- [x] 2.3 Implement `Watch` using the typed `client-go` Events watch with a
  retry/reconnect loop; map each `*corev1.Event` to `events.Event` and call
  `obs.OnEvent`; wrap stream errors with `fmt.Errorf("k8s events: %w", err)`.
- [x] 2.4 Translate context cancellation into stopping the watch and returning
  cleanly with no leaked goroutines.
- [x] 2.5 Add adapter unit tests using a fake clientset / `envtest`: assert the
  observer is notified per matching event, `namespace`/`labels` filters are honored,
  an unsupported filter key errors, and context cancellation stops the watch.

## 3. Dependencies and wiring

- [x] 3.1 Promote `k8s.io/client-go`, `k8s.io/apimachinery`, `k8s.io/api` to direct
  requires in `go.mod`; run `make tidy`.

## 4. Validation

- [x] 4.1 Run `make lint build test` and fix any issues.
- [x] 4.2 Propose a git commit message (do not commit).

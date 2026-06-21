## Context

`kubectl-notify` is a kubectl plugin (cobra + `genericclioptions`) currently at the
bootstrap stage with no domain code. The AGENTS.md mandates a hexagonal architecture:
`internal/port/` holds interfaces, `internal/adapter/` holds implementations, and port
imports must be aliased with a `Port` suffix. This change lays the output side of the
hexagon — a `Notifier` port and a desktop adapter — and a CLI command to drive it. It
is the first vertical slice and sets the package conventions every later capability
(Kubernetes/FluxCD watchers, web UI) will follow.

## Goals / Non-Goals

**Goals:**
- Establish the `internal/port` ↔ `internal/adapter` directory convention.
- Define a minimal, technology-agnostic `Notifier` port.
- Provide a working desktop adapter for macOS/Linux/Windows.
- Provide a `kubectl notify test` command that wires adapter → port → CLI.
- Keep the port free of any concrete dependency so future adapters (web UI, Slack)
  drop in without touching callers.

**Non-Goals:**
- No Kubernetes/FluxCD event sources yet (input side of the hexagon).
- No notification batching, deduplication, templating, or routing.
- No notification icons/sounds/actions beyond what the library gives for free.
- No persistence or configuration file for notifier selection.

## Decisions

### Port shape: `Notifier` with a `Notification` value object
The port is a single-method interface:

```go
// internal/port/notification/notifier.go
package notification

type Urgency int

const (
    UrgencyLow Urgency = iota
    UrgencyNormal
    UrgencyCritical
)

type Notification struct {
    Title   string
    Body    string
    Urgency Urgency
}

type Notifier interface {
    Notify(ctx context.Context, n Notification) error
}
```

`context.Context` is included now so async/cancellable adapters (HTTP, queues) don't
force a breaking signature change later. Rationale: a value object keeps the call site
stable as fields grow; a single method keeps adapters trivial.

**Alternative considered:** separate `Notify(title, body string)` positional args —
rejected because every new field would break the interface.

### Adapter library: `github.com/gen2brain/beeep`
`beeep` is a small, pure-Go, cross-platform desktop notification library (no cgo on
the common paths) covering macOS, Linux (notify-send/DBus), and Windows toasts. It is
widely used and dependency-light.

**Alternatives considered:**
- `github.com/0xAX/notificator` — shells out to platform binaries, less portable.
- Hand-rolled per-OS code — too much surface area for a bootstrap slice.

The dependency is justified per AGENTS.md guardrail #3 (recorded here).

### Urgency mapping & beeep API
Confirmed from the beeep README / godoc, the relevant API is:

```go
func Notify(title, message string, icon any) error  // standard toast
func Alert(title, message string, icon any) error   // attention dialog
var AppName = "DefaultAppName"                        // settable app name
```

`icon` is `any` (a file-path string, embedded `[]byte`, or empty). The adapter passes
an empty icon (`""`) for now — icons are a Non-Goal. The adapter sets
`beeep.AppName = "kubectl-notify"` once at construction. Urgency mapping:
`UrgencyCritical` → `beeep.Alert`, `UrgencyLow`/`UrgencyNormal` → `beeep.Notify`. The
adapter owns this mapping so the port stays platform-neutral.

### Wiring & command placement
The desktop adapter is constructed in `cmd/test.go` and injected into the command —
no global state (AGENTS convention). Port package imported as
`notificationPort "github.com/ebuildy/kubectl-notify/internal/port/notification"`.
A compile-time `var _ notificationPort.Notifier = (*Adapter)(nil)` assertion lives in
the adapter to guarantee conformance.

## Risks / Trade-offs

- **Headless CI cannot show real toasts** → the adapter test asserts the type satisfies
  the port and that `Notify` returns without panicking; actual delivery is verified
  manually / on a desktop. No `t.Skip` without an issue (AGENTS rule).
- **`beeep` brings transitive deps / possible cgo on some platforms** → mitigated by
  pinning the version and keeping it isolated behind the adapter; swapping libraries
  only touches `internal/adapter/notification/desktop`.
- **Linux requires `notify-send`/DBus present at runtime** → adapter returns a wrapped
  error; the CLI surfaces it and exits non-zero (spec scenario covers this).

## Migration Plan

Net-new code on an empty bootstrap; nothing to migrate or roll back. Reverting is a
straight file/dep removal. `make tidy` after adding the dependency.

## Open Questions

- Should notifier selection eventually be flag-driven (`--notifier desktop|web`)? Out of
  scope here; the port is designed to accommodate it later.

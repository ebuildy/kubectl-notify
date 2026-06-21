## 1. Hexagonal skeleton & dependency

- [x] 1.1 Create directory layout: `internal/port/notification/` and `internal/adapter/notification/desktop/`
- [x] 1.2 Add `github.com/gen2brain/beeep` to `go.mod` and run `make tidy`

## 2. Notifier port

- [x] 2.1 Create `internal/port/notification/notifier.go` defining the `Urgency` type with `UrgencyLow`/`UrgencyNormal`/`UrgencyCritical` constants
- [x] 2.2 In the same package, define the `Notification` value object (`Title`, `Body`, `Urgency`) and the `Notifier` interface (`Notify(ctx context.Context, n Notification) error`)

## 3. Desktop adapter

- [x] 3.1 Create `internal/adapter/notification/desktop/desktop.go` with an `Adapter` struct and a `New()` constructor that sets `beeep.AppName = "kubectl-notify"`; the `Notify` method maps urgency (`UrgencyCritical`→`beeep.Alert(title, body, "")`, else `beeep.Notify(title, body, "")`) and wraps delivery errors with adapter context
- [x] 3.2 Add a compile-time assertion `var _ notificationPort.Notifier = (*Adapter)(nil)` using the `notificationPort` import alias
- [x] 3.3 Add `desktop_test.go` verifying the adapter satisfies the port and that `Notify` returns without panicking (no `t.Skip`)

## 4. CLI test command

- [x] 4.1 Create `cmd/test.go` with a `kubectl notify test` subcommand exposing `--title`, `--body`, and `--level` flags (sane defaults; `--level` defaults to `normal`)
- [x] 4.2 Parse `--level` (`low`/`normal`/`critical`) to the port `Urgency`, rejecting unknown values with an error naming the accepted levels before sending
- [x] 4.3 Construct the desktop adapter, send a `Notification` through the `Notifier` port, print errors to stderr and return non-zero on failure, `0` on success
- [x] 4.4 Register the command on the root command in `cmd/root.go`

## 5. Verify

- [x] 5.1 Run `make lint build test` and fix any issues
- [x] 5.2 Manually run `./bin/kubectl-notify test --level low|normal|critical` on a desktop to confirm a toast appears for each level (critical shows an alert)
- [x] 5.3 Propose a git commit message summarizing the change

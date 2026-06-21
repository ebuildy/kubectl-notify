# Desktop Notification

## Purpose

Define the output side of the hexagon for delivering notifications to a user. This
capability provides a technology-agnostic `Notifier` port, a desktop adapter that
renders OS toast notifications, and a CLI command to exercise the port end to end
without a Kubernetes connection.

## Requirements

### Requirement: Notifier port

The system SHALL define a technology-agnostic `Notifier` port (interface) that
accepts a notification consisting of a title, a body, and an urgency level, and
returns an error when delivery fails. The port MUST NOT reference any concrete
notification technology (no OS, library, or transport details).

#### Scenario: Notification carries title, body and urgency

- **WHEN** a caller constructs a notification
- **THEN** it can set a non-empty title, an optional body, and one of the defined urgency levels (`low`, `normal`, `critical`)

#### Scenario: Delivery failure surfaces an error

- **WHEN** a `Notifier` implementation fails to deliver a notification
- **THEN** it returns a non-nil error wrapped with context describing the adapter that failed

### Requirement: Desktop adapter

The system SHALL provide a desktop adapter that implements the `Notifier` port by
displaying an operating-system desktop notification (toast). The adapter MUST map
the port's urgency levels onto the underlying platform's notification semantics and
MUST work on macOS, Linux, and Windows.

#### Scenario: Desktop notification is shown

- **WHEN** the desktop adapter receives a notification with a title and body
- **THEN** an OS desktop notification is displayed with that title and body, and no error is returned

#### Scenario: Adapter satisfies the port

- **WHEN** the desktop adapter is constructed
- **THEN** it is assignable to the `Notifier` port interface (compile-time guarantee)

### Requirement: CLI test command

The system SHALL provide a `kubectl notify test` subcommand that sends a sample
notification through the `Notifier` port using the desktop adapter, so the output
side of the hexagon can be exercised without any Kubernetes connection.

#### Scenario: Test command sends a sample notification

- **WHEN** a user runs `kubectl notify test`
- **THEN** a sample desktop notification is delivered through the port and the command exits with code `0`

#### Scenario: Test command allows overriding title and body

- **WHEN** a user runs `kubectl notify test --title "Hi" --body "World"`
- **THEN** the delivered notification uses the provided title and body

#### Scenario: Test command simulates a given urgency level

- **WHEN** a user runs `kubectl notify test --level low`, `--level normal`, or `--level critical`
- **THEN** the delivered notification carries the matching `Urgency` value through the port

#### Scenario: Invalid urgency level is rejected

- **WHEN** a user runs `kubectl notify test --level bogus`
- **THEN** the command prints an error naming the accepted levels (`low`, `normal`, `critical`) and exits with a non-zero status without sending a notification

#### Scenario: Delivery failure is reported

- **WHEN** the underlying desktop notification fails to deliver
- **THEN** the command prints the error to stderr and exits with a non-zero status

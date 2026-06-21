# event-notification-controller Specification

## Purpose
TBD - created by archiving change add-notification-controller. Update Purpose after archive.
## Requirements
### Requirement: Controller bridges EventSource to Notifier

The system SHALL provide an application-layer controller that implements the
`events.Observer` interface and holds a `notification.Notifier`. The controller
SHALL map each received event to a `notification.Notification` and deliver it
through the `Notifier` port. The controller MUST depend only on the
`EventSource`/`Observer` and `Notifier` ports, and MUST NOT reference any concrete
adapter, OS, or transport type. The controller SHALL be constructed via a
constructor that injects its `Notifier`, a destination for diagnostic logging, the
debounce window duration, and the batch threshold, with no global state.

#### Scenario: Received events become delivered notifications

- **WHEN** the controller receives one or more events and the buffer is flushed
- **THEN** it constructs notifications from those events and calls the injected `Notifier.Notify` accordingly

#### Scenario: Controller satisfies the Observer port

- **WHEN** the controller is constructed
- **THEN** it is assignable to the `events.Observer` interface (compile-time guarantee)

### Requirement: Time-windowed debounce buffering

The controller SHALL NOT deliver a notification synchronously from `OnEvent`.
Instead, `OnEvent` SHALL append the event to an in-memory buffer and return promptly
so the watch goroutine is never blocked by delivery. The controller SHALL expose a
`Run(ctx)` method that, until the context is cancelled, flushes the buffered events
once per configured debounce window; on context cancellation it SHALL perform a
final flush of any remaining buffered events and return. Concurrent access to the
buffer SHALL be synchronized so the `Notifier` is never invoked from more than one
goroutine at a time. A configured window of zero SHALL flush on every event,
disabling buffering.

#### Scenario: OnEvent buffers without delivering

- **WHEN** `OnEvent` is invoked before a flush occurs
- **THEN** it returns without calling `Notifier.Notify`, and the event is delivered only on the next flush

#### Scenario: Buffer is flushed once per window

- **WHEN** `Run` is active with a non-zero window and events have been buffered
- **THEN** the buffered events are delivered together when the window elapses, and the buffer is cleared for the next window

#### Scenario: Final flush on cancellation

- **WHEN** the context passed to `Run` is cancelled while events remain buffered
- **THEN** the controller delivers the remaining buffered events once and `Run` returns

### Requirement: Burst summarisation above the threshold

The controller SHALL choose between individual and summary delivery by volume.
When a flushed window contains a number of events at or below the configured batch
threshold, the controller SHALL deliver each event as its own notification using the
event-to-notification mapping. When a flushed window contains more events than the
threshold, the controller SHALL NOT deliver individual notifications; instead it
SHALL group the events by their cause — the `(Kind, Reason)` pair, not their type or
urgency — and deliver one summary notification per distinct group, whose body states
the count and group in the form `"<count> events of <Kind>/<Reason>"`. The summary's
urgency SHALL be critical when any event in the group is a `Warning`, and normal
otherwise.

#### Scenario: Volume at or below threshold sends individual notifications

- **WHEN** a window flush contains a number of events at or below the threshold
- **THEN** each event is delivered as its own mapped notification

#### Scenario: Volume above threshold sends per-kind/reason summaries

- **WHEN** a window flush contains more events than the threshold, spanning one or more distinct `(Kind, Reason)` groups
- **THEN** the controller delivers exactly one summary notification per distinct `(Kind, Reason)` group, each with a body of the form `"<count> events of <Kind>/<Reason>"`, and delivers no individual notifications for that window

#### Scenario: Summary urgency reflects the group's severity

- **WHEN** a summary group contains at least one `Warning` event
- **THEN** the summary notification carries `UrgencyCritical`; otherwise it carries `UrgencyNormal`

### Requirement: Event to notification mapping

The controller SHALL map an `events.Event` onto a `notification.Notification` using
deterministic rules: the title SHALL combine the referenced object's identity (kind
and name) with the event reason; the body SHALL be the event message; and the
urgency SHALL be derived from the event type, where a `Warning` type maps to
`UrgencyCritical` and any other type (including `Normal` or unknown) maps to
`UrgencyNormal`. The mapping MUST degrade gracefully when individual identity fields
are empty rather than producing an error.

#### Scenario: Warning event maps to critical urgency

- **WHEN** the controller maps an event whose type is `Warning`
- **THEN** the resulting notification has `UrgencyCritical`

#### Scenario: Normal event maps to normal urgency

- **WHEN** the controller maps an event whose type is `Normal` or an unrecognized value
- **THEN** the resulting notification has `UrgencyNormal`

#### Scenario: Title and body carry event identity and message

- **WHEN** the controller maps an event with a kind, name, reason, and message
- **THEN** the notification title includes the object identity and the reason, and the notification body is the event message

### Requirement: Delivery failure is tolerated

The controller SHALL tolerate a delivery failure rather than aborting the flush.
When delivery of a notification through the `Notifier` fails during a flush, the
controller SHALL log the failure to its injected diagnostic destination and SHALL
continue delivering the remaining notifications for that window. A delivery failure
MUST NOT stop the flush loop or the underlying watch.

#### Scenario: Delivery failure does not stop the flush

- **WHEN** the injected `Notifier.Notify` returns an error for one notification in a flush
- **THEN** the controller writes a diagnostic message describing the failure and proceeds to deliver the remaining notifications for that window, and the flush loop and watch keep running


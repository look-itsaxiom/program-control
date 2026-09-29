**STRETCH.** Optional. Read the [primer](./PRIMER.md) first.

# Phase 5: integrations

This phase adds one route to `httpapi.NewHandler`'s handler, reached the same way phase 2's routes
are, plus behavior that runs after a response has already gone out. Nothing here replaces or
narrows any earlier phase's contract.

## S5.1: accept a signed vendor webhook

As a vendor's integration, I need to tell Program Control that a task's status changed on my side,
without a person in the loop and without a session of any kind, so that Program Control's board
stays accurate even when the change happened somewhere else.

**Acceptance:** a request signed with the current shared secret, sent recently, updates the named
task's status and is accepted. A request signed with the wrong secret is refused outright, before
anything about its content is trusted. A request whose timestamp is too old is refused, even if
its signature is otherwise valid, because a valid old signature is exactly what a captured and
replayed request looks like. Sending the exact same message a second time does not change the
task's status again or record a second event; the caller is told the message was already handled.

**Contract:**

- `POST /webhooks/vendor`, headers `X-Timestamp` (the request's creation time, as a decimal Unix
  timestamp in seconds) and `X-Signature` (lowercase hex-encoded HMAC-SHA256, keyed with the
  current webhook secret, of the exact byte string `"<X-Timestamp value>.<raw request body>"`).
  Body: `{"id","type":"task.status","taskId","status"}`, where `id` uniquely identifies this
  message (not the task), `type` is always `"task.status"` for this route, `taskId` names an
  existing task, and `status` is one of `todo`, `doing`, `blocked`, `done` (S2.4's set).
- The current secret comes from the `WEBHOOK_SECRET` environment variable. During a rotation, the
  value of `WEBHOOK_SECRET_PREVIOUS`, when that environment variable is set, is also accepted; a
  request signed with either value is valid.
- `401`, JSON error envelope, when the signature does not match `WEBHOOK_SECRET` or (if set)
  `WEBHOOK_SECRET_PREVIOUS`.
- `400`, JSON error envelope, when `X-Timestamp` is more than 5 minutes away from the server's own
  clock, in either direction, even with a valid signature.
- `200` `{"status":"duplicate"}` when this exact message `id` has already been accepted once
  before (checked after the signature and timestamp are validated, not before, so an attacker
  without a valid secret learns nothing from this shortcut).
- Otherwise: the named task's `status` is updated exactly as `PATCH /tasks/{id}` (S2.4) would
  update it, one `task_events` row is appended with `kind` = `"webhook"`, `from_status` and
  `to_status` set the same way S2.4 sets them, and `actor_id` left unset (there is no person to
  name). Response: `202` `{"status":"accepted"}`.

## S5.2: notify downstream without blocking

As an operator, I need something else to learn about a status change soon after it happens,
without the person or system that caused the change (a vendor's webhook, or a caller of
`PATCH /tasks/{id}`) ever waiting on that notification to go out, so that a slow or unreachable
downstream system is never the reason a status change feels slow or fails.

**Acceptance:** every successful status change, however it was made, eventually reaches the
configured downstream target. The response to whatever request caused the change returns without
waiting for that delivery, even when the downstream target is slow to respond. A downstream target
that fails is retried a bounded number of times, with a growing pause between attempts, rather
than given up on after one failure or retried forever.

**Contract:**

- When the environment variable `WEBHOOK_NOTIFY_URL` is set, every status change accepted by this
  phase's route or by `PATCH /tasks/{id}` (S2.4) is, after the change is durable and the
  triggering request's own response has been written, delivered as an HTTP `POST` to that URL. The
  request body is a JSON object shaped `{"taskId","fromStatus","toStatus","kind","occurredAt"}`,
  where `kind` is `"webhook"` or `"status_changed"` matching the `task_events.kind` the change
  produced, and `occurredAt` is an RFC 3339 timestamp.
- Delivery is attempted up to 3 times total, with a backoff between attempts (the exact durations
  are not part of the contract), and is abandoned, not retried indefinitely, once 3 attempts have
  failed. All three attempts complete within 10 seconds of the status change, so a backoff
  schedule is free to grow between attempts but not to stretch past that bound.
- When `WEBHOOK_NOTIFY_URL` is unset, no delivery attempt happens; nothing about this story blocks
  or fails a status change when nothing is configured to be notified.

## S5.3: background work

As an operator, I need the delivery work from S5.2 to run somewhere other than the request that
triggered it, so that a burst of status changes doesn't leave a pile of goroutines each holding a
request open, and so that one slow delivery never delays a request that has nothing to do with it.

**Acceptance:** delivery work is handed off to run independently of the request that produced it,
in whatever shape (a queue and a worker, a bounded pool, anything else) accomplishes that. Nothing
about how the handoff is implemented is part of the contract; what's graded is only what an outside
observer can see: the response returns promptly regardless of downstream speed, and delivery
(including its retries) still eventually happens.

**Contract:** none beyond S5.2's; this story names the requirement that S5.2's delivery cannot run
inline on the request's own goroutine, without adding a new observable shape of its own.

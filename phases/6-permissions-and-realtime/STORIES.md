**STRETCH.** Optional. Read the [primer](./PRIMER.md) first.

# Phase 6: permissions and realtime

This phase refines who sees what within a program a caller can already reach (S2.8 still decides
whether they can reach the program at all), and adds two read-only routes: one for history, one
for a live feed of changes. Nothing here changes any write route's contract.

## S6.1: vendors see only what they need

As a customer, I need a vendor I've shared a program with to see the tasks that are theirs to work
on, and just enough about what those tasks depend on to know why they're waiting, without seeing
the rest of my program's detail, so that sharing a program with a vendor doesn't mean handing them
the whole thing.

**Acceptance:** a vendor org member sees full detail for every task their own organization owns.
For a task they don't own that one of their own tasks directly or transitively depends on, they
see enough to know it exists, what it's called, and its current status, and nothing more. A task
that is neither owned by their organization nor a predecessor of one of their tasks does not
appear to them at all, the same as if it didn't exist. A customer org member (the program's owning
organization) continues to see everything, exactly as S2.2 and S2.8 already describe; this story
narrows what a *shared* organization's member sees, not what the owner sees.

**Contract:**

- For a caller who belongs to an organization the program is shared with (not the owning
  organization): `GET /programs/{programId}/tasks` (and `/tasks/ready`, which inherits the same
  rule) includes, for every task, either the full `Task` shape from S2.2 (when `orgId` is the
  caller's own organization) or a reduced shape containing only `{"id","title","status"}` (when
  the task is not owned by the caller's organization but is a predecessor, direct or transitive,
  of a task the caller's organization owns). Every other task in the program is left out of the
  list entirely.
- `GET /tasks/{id}` for a task the caller's organization owns, or a predecessor of one it owns,
  answers with the same shape (full or reduced) the list would have shown for that task. For a
  task that is neither, it answers `404`, the same as for a program the caller cannot reach at
  all (S2.8's "don't confirm it exists" rule extends to individual tasks here).
- The reduced shape's JSON object has exactly the three keys named above; every other key from
  the full `Task` shape (`programId`, `orgId`, `estimateMinutes`, `assigneeId`, `dueOn`,
  `internalNote`, `createdAt`, `updatedAt`) is absent, not `null`.

## S6.2: audit trail

As a program member, I need to see the ordered history of what happened to tasks I can see, so
that I can catch up on what changed since I last looked without re-deriving it from current
state alone.

**Acceptance:** a caller can ask for events after a given point and get back everything that
happened since, in the order it happened, limited to events about tasks that caller is allowed to
see at all under S6.1 and S2.8.

**Contract:**

- `GET /programs/{programId}/events?since=<eventId>` -> `200` with `{"events":[Event, ...]}`,
  where `since` is optional (its absence means "from the beginning") and, when present, names a
  `task_events.id`; only events with a strictly greater id are included. Results are ordered
  ascending by `id`.
- Each `Event` is `{"id","taskId","kind","fromStatus","toStatus","actorId","createdAt"}`, drawn
  directly from `task_events` (S1.5): `fromStatus`, `toStatus`, and `actorId` are `null` when
  unset; `id` is the 64-bit integer described in S1.5, serialized as a JSON number.
- Only events for tasks the caller could see (full or reduced, per S6.1) are included; an event
  for a task the caller cannot see at all is left out, not replaced with a placeholder.
- This route follows S2.8's reachability rule for the program itself: a caller who cannot reach
  the program at all gets `404`.

## S6.3: live board

As anyone looking at the board, I need to see a task's status change on my screen without
reloading the page, so that a board with several people watching it stays honest in real time
instead of only being as fresh as the last manual refresh.

**Acceptance:** while a client is connected to a program's live feed, every status change to a
task that client could see (per S6.1) arrives on that connection shortly after it happens,
without the client having to ask again. A client that cannot see a given task at all under S6.1
never receives an event about it.

**Contract:**

- `GET /programs/{programId}/stream`, `Content-Type: text/event-stream`, kept open for the
  connection's lifetime. On every task status change (from any of the write paths in this phase or
  earlier ones: `PATCH /tasks/{id}`, the phase 5 webhook, or later a GraphQL mutation) visible to
  this connection's caller, the server writes one Server-Sent Event: `event: task.updated`,
  `data:` the task's full JSON in whatever shape (full or reduced) S6.1 would give that caller for
  that task via `GET /tasks/{id}`.
- The web client consumes this stream with `EventSource`, constructing one once it knows which
  program and user it is, and updates the affected row when a `task.updated` event names a task it
  is currently displaying, without issuing a new request for the board's data.

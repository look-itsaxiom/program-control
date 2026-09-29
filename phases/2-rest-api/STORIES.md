# Phase 2: REST API

Read the [primer](./PRIMER.md) first. Every story in this phase is delivered inside
`api/internal/httpapi` (or packages it imports), reached through one entry point:

**Contract:** `httpapi.NewHandler(ctx context.Context, cfg Config) (http.Handler, error)`, where
`Config` has at least `DatabaseURL string` and `Logger *slog.Logger`. This is the only symbol the
phase's tests, and every later phase, import from this package; nothing about routers, middleware
composition, or how a handler reaches the database is part of the contract beyond that one
function's signature.

## S2.1: identify the caller

As an API caller, I need to identify myself to the server on every request, so that the rest of
this phase's rules (which programs I can see, whose internal notes I can read, who an audit event
was performed by) have someone to check against.

**Acceptance:** a request that supplies a real user's id is treated as that person for the rest
of its handling. A request that supplies no identity at all is refused before any route's own
logic runs. A request that supplies an identity naming nobody real is refused the same way, not
treated as anonymous and not treated as a lookup failure the route itself has to handle.

**Contract:**

- Every route this phase defines (other than `GET /healthz`, unaffected by this phase) requires a
  request header `X-User-Id: <uuid>`, naming a `users.id` from phase 1's schema.
- A request missing that header is answered `401` with the JSON error envelope
  `{"error":{"code":"unauthenticated","message":"..."}}`.
- A request whose `X-User-Id` value does not match any existing user is answered the same way:
  `401`, `error.code` = `"unauthenticated"`.
- There is no session, cookie, or token anywhere in this phase; the header is the entire
  authentication story, standing in for a real login system a later phase might add.

## S2.2: list a program's tasks

As a program member, I need to list a program's tasks, optionally narrowed to one status, so
that I can see the current state of the board.

**Acceptance:** listing a program's tasks returns every task that program has, excluding any that
are soft-deleted, ordered so that tasks with a due date come before tasks without one, and, among
tasks that share an ordering position, earlier-created tasks come first. Narrowing to a status
returns only tasks currently in that status; leaving the narrowing off returns every status.

**Contract:**

- `GET /programs/{programId}/tasks` and `GET /programs/{programId}/tasks?status=<status>` both
  answer `200` with `{"tasks":[Task, ...]}` (`[]` when the program has none, never `null`).
- Ordering is `due_on` ascending with tasks that have no `due_on` sorted last, then `created_at`
  ascending among rows that tie on `due_on`.
- `status`, when present, narrows the list to tasks whose `status` exactly matches it; an absent
  `status` parameter lists every status.
- Every `Task` in this phase's responses, in every story, has this shape:

  ```json
  {
    "id": "uuid",
    "programId": "uuid",
    "orgId": "uuid",
    "title": "string",
    "status": "todo | doing | blocked | done",
    "estimateMinutes": 30,
    "assigneeId": "uuid",
    "dueOn": "YYYY-MM-DD",
    "internalNote": "string",
    "createdAt": "RFC 3339 timestamp",
    "updatedAt": "RFC 3339 timestamp"
  }
  ```

  `estimateMinutes`, `assigneeId`, and `dueOn` are `null` when unset. `internalNote` is `null`
  when unset for a caller allowed to see it at all, and omitted from the object entirely (the key
  itself absent, not `null`) for a caller outside the task's own organization; see S2.8 for the
  exact rule.
- Who may call this route at all, and what happens for someone outside the program's owning and
  shared organizations, is S2.8's contract, not this one.

## S2.3: create a task

As an editor of a program, I need to create a task with a title and, optionally, an estimate, an
assignee, a due date, and an internal note, so that new work has somewhere to live and I can fetch
it back afterward.

**Acceptance:** creating a task with at least a title succeeds and the new task can immediately be
fetched back by the id the creation response gave. A blank title, a negative estimate, or a due
date that isn't a real calendar date is refused, with an explanation of which field was wrong.
Creating a task whose title exactly matches another still-active task's title in the same program
is refused as a conflict, not silently allowed to produce two tasks with the same name. Fetching a
task by an id nobody has ever refers to, or that has been soft-deleted, answers not-found.

**Contract:**

- `POST /programs/{programId}/tasks`, body `{"title","estimateMinutes"?,"assigneeId"?,"dueOn"?,"internalNote"?}`
  (`title` required; the rest optional). On success: `201`, header `Location: /tasks/{id}`, body
  the created `Task` (shape from S2.2).
- `400`, `error.code` = `"validation"`, body
  `{"error":{"code":"validation","message":"...","details":[{"field":"...","message":"..."}]}}`,
  with one `details` entry per problem, for: an empty or whitespace-only `title`; a negative
  `estimateMinutes`; a `dueOn` that is not a valid `YYYY-MM-DD` calendar date. Each entry's
  `field` is the offending field's own JSON name from the request body (`title`,
  `estimateMinutes`, or `dueOn`).
- `409`, `error.code` = `"duplicate_title"`, when another active (non-soft-deleted) task in the
  same program already has this exact `title`.
- `GET /tasks/{id}` answers `200` with the `Task`, or `404` if no active task has that id.
- The task created this way is owned by the organization the creating caller belongs to (its
  `org_id`); which organization that is when a caller belongs to more than one is left to the
  implementation, since this phase's fixtures never put a caller in more than one organization.

## S2.4: change status with audit

As a program member, I need to change a task's status and have that change recorded, so that
everyone looking at the task later can trust what happened to it and when.

**Acceptance:** changing a task to one of the recognized statuses succeeds and the task reflects
the new status immediately afterward. Changing it to a value outside the recognized set is
refused, and the task is left exactly as it was. Every successful change is recorded permanently,
naming who made it, what it changed from, and what it changed to.

**Contract:**

- `PATCH /tasks/{id}`, body `{"status"}` -> `200` with the updated `Task`.
- `400`, `error.code` = `"validation"`, when `status` is not one of `todo`, `doing`, `blocked`,
  `done`.
- Every successful change appends exactly one row to `task_events`: `kind` = `"status_changed"`,
  `from_status` the status the task had before, `to_status` the new status, `actor_id` the
  caller's id (the `X-User-Id` that made the request).

## S2.5: add and remove dependencies with cycle rejection

As a program member, I need to add and remove dependencies between tasks, and be stopped from
creating a cycle, so that the dependency graph the ready list (S2.6) relies on always stays a
valid ordering with no task waiting on itself, directly or through a chain of other tasks.

**Acceptance:** recording that one task must finish before another succeeds and both tasks
reflect it. Recording a dependency that already exists exactly as given is refused as a
duplicate. Recording a dependency naming a task that doesn't exist is refused as not-found.
Recording a dependency that would create a cycle, whether the new edge directly reverses an
existing one or only closes a cycle through a longer chain of existing dependencies, is refused,
and the refusal explains which tasks the cycle runs through. Removing a dependency succeeds given
the two tasks it was between.

**Contract:**

- `POST /tasks/{id}/dependencies`, body `{"predecessorId"}`, records that `predecessorId` must
  finish before `id` can start. On success: `201`, body `{"predecessorId","successorId"}` (where
  `successorId` is `id` from the path).
- `409`, `error.code` = `"cycle"`, when the new edge would create a cycle, whether directly (the
  new edge is the exact reverse of an existing one) or transitively (through a longer existing
  chain). `error.message` names every task id on the cycle's path, including both ends of the new
  edge.
- `404` when `id` or `predecessorId` does not name an existing, non-soft-deleted task.
- `409`, `error.code` = `"duplicate"`, when this exact `predecessorId`/`id` pair is already
  recorded.
- `DELETE /tasks/{id}/dependencies/{predecessorId}` removes that dependency, if it exists, and
  answers `204` with no body.

## S2.6: ready list

As a vendor engineer, I need to see which of a program's tasks are ready to start right now, so
that I can plan the shift without manually checking every task's dependencies myself.

**Acceptance:** a task is ready exactly when it is still `todo` and every task it depends on is
`done`. A task with no dependencies at all is ready the moment it is `todo`, with nothing to wait
on. A task whose status is `blocked` never appears on the ready list, no matter what its
dependencies look like, because a human has already said it isn't actually workable right now.

**Contract:**

- `GET /programs/{programId}/tasks/ready` -> `200` with `{"tasks":[Task, ...]}`, the same `Task`
  shape and the same tenant rules (S2.8) as S2.2, containing exactly the tasks meeting the rule
  above, in no particular required order beyond what S2.2 already specifies.

## S2.7: request ids and consistent errors

As an operator running this service, I need every request to carry a request id and every error
response to have the same shape, so that I can find one request's log lines by id and a client
can handle any error the same way instead of special-casing each route's failure format.

**Acceptance:** every response, success or failure, carries a request id. When the caller already
sent one, the response carries that exact value back; when the caller didn't, the server invents
one so the response still has it. Calling a route that exists, but with a method it doesn't
accept, is refused with a response naming which methods it does accept. Calling a path this API
never defined at all is refused the same way every other error is: as the shared JSON envelope,
not a blank body or a framework's default error page. No error response in this entire phase ever
looks different from any other in its basic shape.

**Contract:**

- Every response carries `X-Request-Id`. If the incoming request already had that header, the
  response's value is that exact string, unchanged. Otherwise, the server assigns a new value and
  the response carries it.
- A request whose path matches a route this phase defines, but whose method that route does not
  accept, is answered `405` with an `Allow` header listing the methods that route does accept.
- A request whose path matches no route this phase defines is answered `404` with the same JSON
  error envelope every other error in this phase uses.
- Every error response in this phase, from every story above, is `Content-Type: application/json`
  and a body shaped `{"error":{"code":"...","message":"..."}}`, with `details` added only where
  S2.3 says so.

## S2.8: tenant safety

As a program's owner, I need a stranger to be unable to tell the program exists at all, and a
vendor I've shared it with to never see the private notes I keep for my own organization, so that
crossing an organization's boundary never leaks information by accident, only by an explicit
sharing decision.

**Acceptance:** a caller who belongs to neither the program's owning organization nor any
organization it's been shared with gets exactly the same answer for that program's routes as they
would for a program that was never created at all: not-found, never a "forbidden" that would
confirm something is there to be forbidden from. A caller who does belong to a shared
organization can see the program's tasks, including ones owned by a different organization than
their own, but on any task they don't own, the private note is not just hidden as `null`, its key
is missing from the JSON entirely, so a client can't even tell whether one was ever written.

**Contract:**

- `GET /programs/{programId}/tasks`, `GET /programs/{programId}/tasks/ready`,
  `POST /programs/{programId}/tasks`, and `GET /tasks/{id}` for a task inside that program all
  answer `404` (never `403`) to a caller who is not a member of the program's owning organization
  and not a member of any organization the program has been shared with (at either the `editor`
  or `viewer` share role).
- In every `Task` JSON this phase returns, from any route, `internalNote` is present (as a string,
  or as `null` when unset) when the caller belongs to that task's own owning organization
  (`tasks.org_id`), and is absent from the JSON object entirely, key and all, when the caller
  belongs to a different organization, including a different organization the program is shared
  with.

# Hints: phase 2

Hidden from the learner. Read only by the mentor. Give one rung at a time, only when asked, and
only after the learner has tried the step themselves. Rung 3 describes the shape of a solution in
prose; it never contains code, and it should still leave the learner writing the Go.

## S2.1: identify the caller

1. Where does "who is calling" need to be known: inside every handler separately, or once, before
   any handler runs? If it's the latter, what Go feature from 103 lets one piece of code wrap
   every route without every route calling it explicitly?
2. Course lesson `103-http-services-in-go/03-middleware-and-structure.md`, the "Auth middleware:
   put a principal in context" section, is close to exactly this shape: read a header, look up who
   it names, reject early if that fails, otherwise stash the result somewhere later handlers can
   read it from.
3. Write one middleware that reads `X-User-Id`, looks the value up against `users`, and either
   writes the `401` envelope and returns without calling the wrapped handler, or stores the
   looked-up user (or just its id) in the request's context with a typed, unexported context key,
   then calls the wrapped handler with that updated context. Wrap every route this phase defines,
   except `/healthz`, in that middleware once, rather than repeating the check per handler.

## S2.2: list a program's tasks

1. The ordering rule ("due date ascending, no-due-date last, then created date") is a SQL `order
   by` clause, not something to sort in Go after fetching. What SQL expression puts null values
   last regardless of ascending or descending order?
2. Course lesson `103-http-services-in-go/01-a-server-is-a-function.md` covers reading a path
   wildcard (`r.PathValue`) and a query parameter (`r.URL.Query().Get`) for the `status` filter;
   neither needs anything beyond what that section already shows.
3. The handler reads `programId` from the path and an optional `status` from the query string,
   runs one query against `tasks` scoped to that program (excluding soft-deleted rows) with an
   `order by` on `due_on nulls last, created_at`, filtered additionally by `status` when it was
   given, and maps each row into the `Task` JSON shape before writing `{"tasks": [...]}`. Whatever
   function turns a row into JSON is worth writing once and reusing everywhere this phase returns
   a `Task`, since every other story in this phase returns the same shape.

## S2.3: create a task

1. What's the difference between "this JSON is malformed" and "this JSON parsed fine, but a value
   in it is invalid"? Which of this story's three validation cases (blank title, negative
   estimate, bad date) are really about a value being invalid rather than the JSON itself?
2. Course lesson `103-http-services-in-go/01-a-server-is-a-function.md` covers decoding a JSON
   body and writing a `Location` header on a `201`; the tasks-api exercise under that same lesson
   (`02-tasks-api`) builds a smaller version of exactly this route, worth looking at for the shape
   even though this project's contract differs in its exact fields.
3. The handler decodes the body, validates each optional field in turn, building up a `details`
   slice of `{field, message}` as it goes rather than stopping at the first problem, and answers
   `400` with all of them at once if that slice is non-empty. On success, it inserts the task
   (catching the database's own duplicate-title constraint from phase 1 and mapping that specific
   failure to the `409 duplicate_title` response, rather than a generic `500`), then writes `201`
   with `Location` set to `/tasks/{id}` and the same row-to-JSON function S2.2 uses.

## S2.4: change status with audit

1. What has to happen inside the same transaction as the status update, so that a task never ends
   up changed without the audit row that says it changed, or vice versa?
2. Course lesson `104-go-service-patterns/01-shaping-a-service.md`, "Error handling patterns,"
   covers mapping a domain error (an unrecognized status here) to a transport code once, at the
   edge, which is the same shape this route needs for its `400 validation` response.
3. The handler validates `status` against the fixed set before touching the database at all,
   answering `400` immediately if it's not recognized. On a valid value, it reads the task's
   current status, updates the row, and inserts a `task_events` row recording the actor (from the
   context S2.1's middleware populated), the old status, and the new one, doing both writes
   together so neither can happen without the other.

## S2.5: add and remove dependencies with cycle rejection

1. A new edge `predecessorId -> id` creates a cycle exactly when a path already exists from `id`
   back to `predecessorId` through the existing dependency graph. What's the smallest search you
   could run, starting from `id` and following existing dependency edges forward, to answer "can I
   reach `predecessorId` from here"?
2. Course lesson `104-go-service-patterns/01-shaping-a-service.md` covers structuring a service in
   layers with small interfaces; think about whether the cycle check belongs inside the same SQL
   statement that inserts the row, or as a separate read-then-decide step in Go before the insert,
   and what a recursive query (`with recursive`) buys you if you'd rather do it in one round trip.
3. Before inserting a new `predecessor_id -> successor_id` row, walk the existing graph forward
   from the proposed successor; if that walk ever reaches the proposed predecessor, refuse with
   `409 cycle` and a message built from the path the walk found (the sequence of task ids it
   followed to get there, plus the new edge that would close the loop). A direct cycle is just the
   one-step case of this same walk, so it doesn't need separate handling. Check for an existing
   identical row first and answer `409 duplicate` before running the cycle walk at all, and check
   both task ids exist before either.

## S2.6: ready list

1. For one task, what two things do you need to know to decide if it's ready: its own status, and
   what about its predecessors specifically?
2. This is a query-shape question more than a Go-code question; think about how you'd phrase "a
   task where every row in `task_dependencies` pointing at it (if any) points at a task whose
   status is `done`" as SQL, including the "if any" part for a task with no predecessors at all.
3. A task belongs on the ready list when its own `status` is `todo` and there is no predecessor of
   it (in `task_dependencies`) whose task is not `done`; phrased as a `not exists` subquery against
   `task_dependencies` joined back to `tasks`, that condition is true for a task with zero
   predecessors for the same reason an empty set trivially satisfies "all of these are true." The
   `blocked` exclusion falls straight out of the `status = 'todo'` condition; it needs no separate
   case.

## S2.7: request ids and consistent errors

1. Where does a request id need to be decided: inside the handler for each route, or once, before
   any route-specific code runs, the same way S2.1's caller identification does?
2. Course lesson `103-http-services-in-go/03-middleware-and-structure.md`, "Request IDs and
   structured logging," shows this almost verbatim: read an incoming header, generate one if it's
   absent, set it on the response, and put it in context for anything downstream (a logger, later)
   that wants it.
3. One middleware, wrapping every route including `/healthz`, reads `X-Request-Id` from the
   incoming request, generates a value if it's missing, sets it on the response header before any
   handler writes a body, and stores it in context. The catch-all route this project's skeleton
   already registers for unmatched paths becomes this phase's `404` envelope instead of the
   `501` stub; `http.ServeMux` already answers `405` with `Allow` on its own for a wrong method
   against a registered pattern, so that part needs no code from you at all, only registering the
   routes with explicit methods the way the skeleton already does for `/healthz`.

## S2.8: tenant safety

1. "Is this caller allowed to see this program at all" and "does this caller see this particular
   task's internal note" are two different checks running at two different scopes (once per
   request versus once per task). Where does each naturally belong?
2. This phase's primer, "Why 404, not 403, across a tenant boundary," is written for this story
   directly; re-read it with this story's acceptance criteria in hand.
3. Before any of S2.2's, S2.3's, or S2.6's own logic runs for a given program, check whether the
   caller's organization is either that program's `org_id` or appears in its `program_shares`; if
   neither, answer `404` immediately, the same `404` a nonexistent program id would get, and stop.
   Separately, wherever a `Task` row becomes JSON, compare the caller's organization to that row's
   own `org_id` and leave the `internalNote` field out of the object entirely (not merely `null`)
   when they don't match; a raw `map[string]any` or a second, note-less struct for that case is
   simpler than trying to make one struct's `json` tags conditionally omit a field at encode time.

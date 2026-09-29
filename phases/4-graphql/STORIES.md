# Phase 4: GraphQL

Read the [primer](./PRIMER.md) first. This phase adds a second way to read and write the same
domain phases 1-3 built: a GraphQL layer served from the same API process, alongside (not instead
of) the REST endpoints from phase 2. Nothing in phase 2 or phase 3's REST-backed behavior changes;
the UI gains a second transport it can switch to, not a replacement for the first.

## S4.1: expose the graph

As a client of this API, I need a single GraphQL endpoint that answers who I am and enforces the
same identity rule the REST API already does, so that adding a second transport does not mean
inventing a second way to be signed in.

**Acceptance:** a `POST /graphql` request on the same handler phase 2 already serves, carrying the
same `X-User-Id` header the REST API reads, resolves `{ me { id displayName email } }` to the
caller identified by that header. A request with no `X-User-Id`, or one naming a user nothing
knows about, fails the same way REST's missing-or-unknown header already fails conceptually
(`unauthenticated`), except expressed the way GraphQL expresses failure: an error in the
response's `errors` array carrying `extensions.code: "UNAUTHENTICATED"`, not an HTTP status code
by itself.

**Contract:**

- `POST /graphql`, served by the same handler as phase 2's REST routes, behind the same
  `X-User-Id` header rule.
- The schema below (its exact shape is the contract; everything after this phase depends on
  these field and type names, not on how any of them is resolved):

  ```graphql
  type Query {
    program(id: ID!): Program
    me: User!
  }

  type Mutation {
    setTaskStatus(taskId: ID!, status: TaskStatus!): Task!
    addDependency(taskId: ID!, predecessorId: ID!): Task!
  }

  type Program {
    id: ID!
    name: String!
    tasks(status: TaskStatus): [Task!]!
    readyTasks: [Task!]!
  }

  type Task {
    id: ID!
    title: String!
    status: TaskStatus!
    estimateMinutes: Int
    dueOn: String
    internalNote: String
    assignee: User
    predecessors: [Task!]!
    program: Program!
  }

  type User {
    id: ID!
    displayName: String!
    email: String!
  }

  enum TaskStatus {
    TODO
    DOING
    BLOCKED
    DONE
  }
  ```

- A request with no `X-User-Id`, or an unknown one, produces a GraphQL error whose
  `extensions.code` is `"UNAUTHENTICATED"` (see S4.6 for the full set of codes this phase uses).

## S4.2: the board in one request

As the client, I need one GraphQL request to describe a program together with its tasks, each
task's assignee and predecessors, and which tasks are ready to start, so that rendering the board
does not require a waterfall of separate REST calls the way phase 3's `fetch` calls sometimes did.

**Acceptance:** `program(id)` resolves the named program's `id`, `name`, `tasks` (each with its
own `id`, `title`, `status`, `estimateMinutes`, `dueOn`, `internalNote`, `assignee`, and
`predecessors`, all in the same request), and `readyTasks`. An id naming a program that does not
exist, or that the caller cannot see (the same ownership-or-share rule phase 2 already enforces),
fails with `extensions.code: "NOT_FOUND"` rather than resolving to `null` silently. A caller
outside a task's owning organization sees that task's `internalNote` as `null`; this is the same
visibility rule phase 2's REST API enforces by omitting the JSON key entirely, expressed the way
GraphQL expresses "you may ask for this field, but you may not see its value here": the field is
still present in the response, its value is `null`. `readyTasks` uses the same readiness rule
phase 2 defined: status `todo`, and every predecessor `done` (a task with no predecessors is
ready; a `blocked` task is never ready no matter its predecessors).

**Contract:** the `Program` and `Task` shapes in S4.1's schema block; `NOT_FOUND` as the code for
an id the caller cannot resolve; `internalNote` resolves to `null`, not an error and not an
omitted field, for a caller outside the task's owning organization.

## S4.3: mutate through the graph

As the client, I need to change a task's status and add a dependency through the same graph I read
it from, so that a write and the read that follows it share one mental model of the data instead
of two separate APIs that happen to describe the same tables.

**Acceptance:** `setTaskStatus(taskId, status)` changes the named task's status and returns the
updated `Task`; exactly as phase 2's `PATCH /tasks/{id}` already does, this appends a
`task_events` row (`kind: "status_changed"`, the status moved from and to, and the caller as
actor). `addDependency(taskId, predecessorId)` adds a predecessor edge and returns the updated
successor `Task`; exactly as phase 2's `POST /tasks/{id}/dependencies` already does, a dependency
that would close a cycle (directly, or through a longer chain) is refused, not created, and the
failure carries `extensions.code: "CYCLE"`.

**Contract:** the `Mutation` shape in S4.1's schema block; `setTaskStatus` appends a `task_events`
row the same way phase 2's status-change endpoint does; `addDependency` rejects a cycle with
`extensions.code: "CYCLE"`.

## S4.4: no N+1

As an operator, I need loading a program's board to cost a small, fixed number of database
queries no matter how many tasks it has, so that the product does not get slower every time a
program grows, the way a resolver that queries once per task would.

**Acceptance:** resolving `program { tasks { assignee { displayName } predecessors { title } } }`
for a program with twelve tasks, each with a real assignee and at least one real predecessor,
issues no more than four SQL statements in total, not four per task and not one plus one per task.
This is observed, not asserted from the resolver's source: a query tracer counts every statement
the handler's database connections actually execute during the one request.

**Contract:**

- Loading `program { tasks { assignee { displayName } predecessors { title } } }` for a program
  with 12 tasks issues at most 4 SQL statements in total.
- The handler installs `Config.QueryTracer`, when it is set, on every database connection it
  opens, so this can be observed from outside the handler rather than asserted about its internal
  structure.

## S4.5: Apollo on the client

As Chase's UI, I need a way to load and update the same board through GraphQL instead of REST,
with Apollo's cache doing the work of keeping every view of a task consistent, so that a status
change or a new dependency updates everywhere that task is shown without the UI asking the server
again just to find out what it already knows.

**Acceptance:** when `localStorage['programControl.transport']` is exactly the string `"graphql"`,
the board loads through `POST /graphql` instead of the REST endpoints phase 3 built against; when
the key is unset or anything else, the board keeps using REST (phase 3's own tests, which never
set this key, must keep passing unmodified). Changing a task's status sends the `setTaskStatus`
mutation and the row reflects the new status; nothing about that interaction issues a second
`program` query beyond the one the initial render made, because Apollo's cache absorbs the
mutation's own response and every view reading that task's status re-renders from it. When a task
that another task depends on is marked done, and its sibling predecessor is later marked done too,
a downstream view of the second task's readiness (for example, its "Ready" badge) reflects that
without either mutation triggering an extra request that re-fetches the board to find out.

**Contract:**

- The transport switch: `localStorage['programControl.transport'] === 'graphql'` selects
  GraphQL; anything else (including the key being unset) keeps the board on REST, unchanged from
  phase 3.
- A write through the graph (S4.3) keeps every view built from the same underlying entity
  consistent without an additional request re-fetching the board to reconcile it.

## S4.6: errors with codes

As the client, I need every kind of failure this API's GraphQL layer can produce to carry a
stable, machine-checkable code, so that the UI can tell "you are not signed in" apart from "that
would create a cycle" apart from "that input was invalid" without parsing an error message meant
for a human.

**Acceptance:** every GraphQL error this phase's mutations and queries can produce carries an
`extensions.code` drawn from a fixed, known set (see S4.1's `me`, S4.2's `program`, and S4.3's two
mutations for where each applies). When the UI receives one, it surfaces the failure to the person
using it through the same accessible, announced channel phase 3 already built for a failed REST
write (an `aria-live="polite"` region), not as a silent no-op and not as an uncaught exception.

**Contract:** `extensions.code` is one of exactly `UNAUTHENTICATED` (S4.1), `NOT_FOUND` (S4.2),
`CYCLE` (S4.3), or `VALIDATION` (an input GraphQL's own type system does not already rule out,
such as an empty title, wherever this phase's mutations accept one); a GraphQL error the client
receives is announced through the same live region phase 3's REST error handling already uses.

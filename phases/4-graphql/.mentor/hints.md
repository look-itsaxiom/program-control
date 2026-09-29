# Hints: phase 4

Hidden from the learner. Read only by the mentor. Give one rung at a time, only when asked, and
only after the learner has tried the step themselves. Rung 3 describes the shape of a solution in
prose; it never contains code, and it should still leave the learner writing the resolvers.

## S4.1: expose the graph

1. The `X-User-Id` header already means something to phase 2's REST handlers. Does a GraphQL
   request need a new way to authenticate, or does it just need to reach the same check?
2. `110-graphql-servers-in-go/01-schema-first-design` covers wiring `gqlgen`'s generated
   `NewExecutableSchema` into an `http.Handler` you mount alongside existing routes, not instead
   of them.
3. Mount the generated GraphQL handler at `POST /graphql` on the same mux phase 2's routes live
   on. Read `X-User-Id` the same way an existing REST handler does, look the user up the same
   way, and store the result somewhere a resolver can reach it (Go's `context.Context`, threaded
   through by a small piece of middleware in front of the GraphQL handler, is the usual place). A
   resolver that finds nothing there for `me`, or for any field that requires a caller, returns a
   `*gqlerror.Error` with `Extensions: map[string]any{"code": "UNAUTHENTICATED"}` rather than a
   plain Go error.

## S4.2: the board in one request

1. `program`'s contract says an id the caller cannot see should fail with `NOT_FOUND`, not
   resolve to `null`. Is "the program doesn't exist" and "the program exists but this caller
   can't see it" one check or two, given what phase 2's REST `GET` already does for the same
   distinction?
2. `110-graphql-servers-in-go/01-schema-first-design` shows how `gqlgen` scaffolds one resolver
   function per field once you fill in `schema.graphqls`; each of `Task.assignee`,
   `Task.predecessors`, `Task.internalNote`, `Program.tasks`, and `Program.readyTasks` becomes its
   own function you write, called once per task per request until S4.4 changes that.
3. `Task.internalNote`'s resolver compares the field's own task's `org_id` against the caller's
   organization (the same comparison phase 2's REST JSON-shaping code already makes to decide
   whether to include the key) and returns `nil` instead of the note when they don't match.
   `Program.readyTasks` reuses the same readiness rule (status `todo`, every predecessor `done`)
   phase 2's `GET .../tasks/ready` already computes; if you wrote that as a reusable query or
   function in phase 2, this resolver calls it rather than re-deriving the rule.

## S4.3: mutate through the graph

1. `setTaskStatus` and `addDependency` are meant to produce the same side effects phase 2's
   `PATCH /tasks/{id}` and `POST /tasks/{id}/dependencies` already produce. Could a mutation
   resolver just call the same internal function phase 2's handler calls, rather than
   reimplementing the status-change-plus-event-append logic a second time?
2. `110-graphql-servers-in-go/02-resolvers-and-the-execution-tree` covers mutation resolvers
   specifically: unlike a query field, a mutation field's resolver is expected to have a side
   effect, and gqlgen generates its stub the same way it generates a query field's.
3. Have `setTaskStatus` and `addDependency`'s resolvers call the same domain-layer functions
   phase 2's REST handlers call (extract that logic out of the handler in phase 2 if you haven't
   already, so both transports share it), and translate whatever error that shared function
   returns for a cycle into a `*gqlerror.Error` with `extensions.code: "CYCLE"`.

## S4.4: no N+1

1. Run the query from S4.2 for a program with several tasks and watch what your database logs (or
   a debug print in your data-access code) show. How many statements actually ran, and does that
   number grow if you add another task?
2. `110-graphql-servers-in-go/03-the-n-plus-one-problem` names exactly this symptom;
   `04-batching-with-dataloaders` is the fix, including the "one batch function, called once per
   tick, given every key requested so far" shape a dataloader has. `108-indexes-and-explain/03-
   observability-you-can-query` is the other half: wiring `Config.QueryTracer` onto every
   `pgxpool.Config.ConnConfig.Tracer` (or equivalent) your handler sets up, so the count in S4.4's
   test reflects reality instead of your assumptions about it.
3. Give `Task.assignee` and `Task.predecessors` each their own dataloader, keyed by task id (for
   assignee) or built from a single `select ... where predecessor_id = any($1)` (or the successor
   side, depending on which direction you query) for predecessors, and construct both loaders once
   per incoming request, not once per process. Wire `cfg.QueryTracer` into whatever constructs your
   `pgxpool.Pool` (or per-request connection) so it applies to every connection the handler ever
   opens, not just the ones the phase 4 code path happens to touch.

## S4.5: Apollo on the client

1. Phase 3's `App` already reads a program id from the URL and a user id from `localStorage`.
   What's the smallest change that makes it also check `localStorage['programControl.transport']`
   and choose which client (REST `fetch` calls, or an Apollo `useQuery`) to render the board with?
2. `111-apollo-client-in-react/01-normalized-caching` covers what `ApolloClient`'s `InMemoryCache`
   needs from your queries (an `id` field, consistently, on every type that has one) to normalize
   entities the way S4.5's contract assumes. `02-mutations-and-the-cache` covers what a mutation's
   own selection set needs to include (usually just the fields you changed, plus `id`) for its
   result to update every other query result holding the same entity, without you writing an
   `update` function by hand. If a component reads `readyTasks` from one query and a task's
   `status` from a different one, and a mutation changes that task's status, does the
   `readyTasks` list know to reconsider that task without an extra request? Think about what
   would have to be included in the query that first fetched the board for that to be possible.
3. Read the transport flag once, at the top of `App` (or wherever the board decides which client
   to construct), and pick between a small REST-backed data layer (already built in phase 3) and
   an `ApolloClient` pointed at `/graphql`. For the cache-normalization behavior in S4.5's
   acceptance criteria, fetch `readyTasks` and `tasks` in the same initial query (so their task
   objects are the same normalized cache entries, not separately-fetched copies), and let
   `setTaskStatus`'s own selection request enough of the changed task's fields that Apollo's
   default merge behavior updates it everywhere on its own.

## S4.6: errors with codes

1. `gqlgen` (and the underlying `gqlparser`) already has a type for an error carrying structured
   extensions. Have you looked at what `graphql.AddErrorf` or constructing a `*gqlerror.Error`
   directly actually accepts, rather than returning a plain `error` from a resolver and hoping
   something downstream adds a code?
2. `077-error-handling-conventions/02-structured-errors-over-prose` is the general case;
   `111-apollo-client-in-react/03-reading-errors-from-a-response` covers reading
   `error.graphQLErrors[0].extensions.code` back out on the client, which is what feeds the same
   `aria-live` region phase 3 built for a failed REST write.
3. Every place this phase's resolvers currently return a plain Go error for a client-facing
   failure (unauthenticated, not found, cycle, validation) should instead construct a
   `*gqlerror.Error` (or use gqlgen's error presenter hook to attach `extensions.code` centrally,
   based on a sentinel error type each of those four cases returns) so every failure this phase
   can produce ends up with exactly one of the four codes. On the client, catch the GraphQL error
   from Apollo's result, read its first `extensions.code`, and route it to the same live region
   component phase 3 already renders for a failed status change.

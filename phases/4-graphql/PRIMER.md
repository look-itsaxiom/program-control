# Primer: GraphQL

## Schema-first design

A GraphQL API starts from its schema, not from its resolvers. You write the SDL (the type
definitions in S4.1's contract block) as a description of the graph a client can ask questions
of, before a single resolver function exists to answer any of them; a code generator (`gqlgen`,
which this project's scaffold already wires up in `api/internal/graph`) reads that SDL and
produces the types and interfaces your resolvers implement. This is the opposite order from
writing a REST handler, where the request and response shapes usually live implicitly in the
handler's own code. The payoff of writing the schema first is that it becomes a contract two
people (or, here, a phase's tests and a later phase's UI) can agree on before either of them has
written an implementation; the payoff of generating resolver stubs from it is that a field you
forget to implement is a compile error, not a runtime surprise the first time a client asks for
it. Course lesson `110-graphql-servers-in-go`, section `01-gqlgen-schema-first`, walks through
this workflow directly, including what `gqlgen generate` actually produces and why the generated
code is meant to be regenerated, not hand-edited.

## Resolvers are just functions

Underneath the generated scaffolding, a resolver is an ordinary function: given a parent value
(the object the field belongs to) and the field's arguments, it returns the field's value, the
same shape of work a REST handler's response-building code already does. The difference that
matters is that a GraphQL query can ask for many fields, on many types, nested arbitrarily deep,
in one request, and each of those fields gets its own resolver call; a naive resolver for
`Task.assignee` that queries the database every time it runs works correctly for a single task
and becomes the next section's problem the moment a client asks for a list of them. Course lesson
`110-graphql-servers-in-go`, exercise `02-resolvers-and-dataloader`, has you write these resolver
functions yourself against a schema of the same shape, which is the mental model you need before
the N+1 problem makes sense.

## The N+1 problem and dataloaders

Resolve `program { tasks { assignee { displayName } } }` the naive way, and you get one query for
the program, one for its tasks, and then one more for every single task's assignee: for a program
with a dozen tasks, that is fourteen queries to answer one request, and the number keeps growing
with the program. This is the N+1 problem, and it is the single most common performance mistake
in a hand-written GraphQL server, precisely because each resolver, considered on its own, looks
correct. The fix is not to make each resolver smarter about caching; it is to change when the
underlying query runs at all: a dataloader collects every "give me the assignee for task X"
request that happens within one tick of the event loop, and issues a single batched query (`where
id = any($1)`, or the Go equivalent) once, then hands each caller its own row back out of the
batch. The same technique applies to predecessors. Getting this right is what S4.4's query-count
bound is actually testing; a program with twelve tasks answering in four queries or fewer is only
possible if assignees are batched across tasks, and predecessors are too. Course lesson
`110-graphql-servers-in-go`, section `03-resolvers-under-load`, derives the problem from first
principles with the exact shape this project's schema has; its exercise
`02-resolvers-and-dataloader` covers the batching technique itself, including the "one tick"
batching window and why a dataloader's cache is scoped to a single request, never shared across
requests.

## Field-level visibility, not query-level

Phase 2's REST API hides a field it will not show a caller by leaving the JSON key out entirely.
GraphQL has no equivalent of "the key is missing": every field the client selected appears in the
response, so "you may not see this" has to be expressed as the field resolving to `null` (when
the field's type allows it, as `Task.internalNote` does) instead of the value it would otherwise
hold. This changes where the access-control decision lives: instead of one branch in a handler
that decides whether to include a key, it is a branch inside the field's own resolver, run once
per task per request, deciding what to return for that one field, for that one caller, right now.
The upside of pushing the decision this low is that it composes for free: a task nested three
levels deep, reached through some path you did not anticipate when you wrote the visibility rule,
still gets the same check, because the same resolver runs for it regardless of how the client's
query got there.

## Apollo's cache as an extension of your app's state

A REST client that wants "the row I just changed and the summary count that includes it to agree"
has to either refetch both, or hand-write the logic that updates both in place after a write.
Apollo Client's cache normalizes every object it receives by its `__typename` and `id`
(`Task:abc123`, `User:def456`), which means the exact same entity, reached through two different
queries (or through a mutation's own response), is stored once, not twice; every component
watching a query that includes that entity re-renders from the single shared copy the moment it
changes, with no code written to propagate the update by hand. This is worth treating as a design
decision, not a free lunch: it only works if your queries and mutations actually request `id` (so
the cache has something to normalize by) and enough of an object's fields that the merge produces
a complete picture, not a partially-overwritten one. Course lesson `111-apollo-client-in-react`,
section `03-the-cache-is-the-state-manager`, is the concrete walkthrough of this mechanism; lesson
`77-graphql-clients`, section `03-mutations-optimism-and-architecture`, covers specifically what a
mutation's own selection set needs to include for its result to update every other place that
data is shown, which is the crux of S4.5's cache-normalization story.

## Two transports, one domain

This phase does not replace phase 3's REST-backed UI; it adds a second way to reach the same
data, selected by a runtime flag (`localStorage['programControl.transport']`) rather than a
build-time choice, specifically so both can be exercised and both can keep working. Building a
second transport on top of an already-working one is a useful forcing function: any assumption
your REST-era UI code baked in about how data arrives (one big response, no field-level nulls,
errors as HTTP status codes) has to get named and either generalized or duplicated, and naming
those assumptions is usually more valuable than the two-transports feature itself. Course lesson
`108-indexes-and-explain`, section `01-how-postgres-decides`, is not about GraphQL at all, but it
is where `Config.QueryTracer` (S4.4) comes from as a pattern: instrument the boundary
(here, every connection the handler opens) rather than trying to infer behavior from source code
alone, and you can verify a performance contract instead of arguing about whether a resolver
"should" be efficient.

## Errors are data too, not just an HTTP status

REST leans on HTTP status codes to say what kind of thing went wrong; GraphQL always answers `200
OK` (the request was understood and executed) and puts failure into the response body's `errors`
array instead, one entry per thing that went wrong, each with a `message` meant for a human and an
`extensions` object meant for code. Leaving `extensions` empty and putting everything in
`message` (or worse, encoding the reason as a prefix inside the message string) pushes every
caller back to parsing prose to decide what to do next, which is exactly the failure mode a REST
API's status code plus a `code` field in its JSON body was designed to avoid; a `extensions.code`
that names a fixed, known set of failure kinds (S4.6's four) keeps that same guarantee for
GraphQL. Course lesson `110-graphql-servers-in-go`, section `01-gqlgen-schema-first`, covers
constructing these errors with `gqlerror` and its `Extensions` map; it is worth reading with
REST's error envelope (phase 2's `{"error": {"code": ...}}`) in mind, since S4.6 is the same idea,
ported.

## Questions to answer for yourself

1. If you added a `Dataloader` for `Task.assignee` but not for `Task.predecessors`, what would
   S4.4's query-count test tell you, and would it tell you which of the two fields you forgot?
2. `internalNote` is nullable in the schema; `title` is not (`String!`). What would have to be
   true about your resolver for `title` for the schema itself to guarantee a client can never
   receive `null` there, no matter what a caller is allowed to see?
3. Apollo's cache merges an entity across every query and mutation response that mentions its
   `id`. What could go wrong if two different queries requested different, non-overlapping subsets
   of a `Task`'s fields, and a component depended on a field neither of those two queries happened
   to ask for?

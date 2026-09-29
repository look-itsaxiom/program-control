# Hints: phase 6 (stretch)

Hidden from the learner. Read only by the mentor. Give one rung at a time, only when asked, and
only after the learner has tried the step themselves. Rung 3 describes the shape of a solution in
prose; it never contains code, and it should still leave the learner writing the Go and SQL.

## S6.1: vendors see only what they need

1. "Predecessor, direct or transitive" is the same shape of question S2.5's cycle check already
   had to answer (walking `task_dependencies` edges). What did that walk look like, and can the
   same traversal, run backward, answer "which tasks does this one depend on, through however many
   hops"?
2. Course lesson `112-system-design-cross-org-collaboration/01-visibility-as-a-first-class-query`
   covers building a query around "what is this caller allowed to see" as a first-class parameter,
   rather than fetching broadly and filtering the response afterward. A recursive CTE (`with
   recursive`) is the natural SQL shape for "every predecessor, however many hops away," if you
   haven't used one yet.
3. For a caller in a shared (non-owning) organization, run two queries: the tasks their own
   `org_id` owns, full shape, and a recursive walk of `task_dependencies` from those tasks'
   predecessor edges outward, collecting every task reached, reduced to `id`, `title`, `status`.
   Union the two sets before returning, and use two different response structs (or a struct with
   most fields as unexported/omitted based on a caller-visibility flag) so the reduced shape's
   missing keys come from the struct genuinely not having those fields for that response, not from
   nulling them out.

## S6.2: audit trail

1. Given the query you just wrote for S6.1 (which tasks can this caller see), how would you use
   its result as a filter on `task_events.task_id` for this story?
2. Nothing new conceptually; this is `where id > $since order by id asc` over `task_events`,
   joined or filtered against the visible-task-ids from S6.1.
3. Reuse whatever internal helper computes "the set of task ids this caller can see" from S6.1;
   this route's query is that set intersected with `task_events.task_id`, `id > $since`, ordered by
   `id`.

## S6.3: live board

1. An HTTP handler function only knows about the one request it's currently handling. What does
   the rest of your server need to hold onto so that a status change happening inside a completely
   different request (a `PATCH`) can reach every open stream connection that cares about it?
2. Course lesson `112-system-design-cross-org-collaboration/02-long-lived-connections` covers the
   registry-plus-broadcast shape directly: a small in-process struct holding a set of currently
   open connections (channels, one per connection), and a `Broadcast(event)` method every write
   path calls after a successful change, which fans the event out to each registered channel.
   `net/http`'s `http.Flusher` interface is what lets a handler push bytes to the client
   immediately instead of buffering until the handler returns.
3. Keep a registry (a mutex-guarded map or slice of channels) inside the package. The
   `GET /programs/{id}/stream` handler creates a channel, registers it (filtered to events this
   caller and this program can see), writes SSE-formatted lines as `task.updated` events arrive on
   it, and deregisters it when the request's context is done (the client disconnected). Every
   write path that changes a task's status calls the registry's broadcast method after its own
   change is durable, the same "after, not during" ordering S5.2 already established for the
   notifier.

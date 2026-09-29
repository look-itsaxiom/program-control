# Hints: phase 3

Hidden from the learner. Read only by the mentor. Give one rung at a time, only when asked, and
only after the learner has tried the step themselves. Rung 3 describes the shape of a solution in
prose; it never contains code, and it should still leave the learner writing the components.

## S3.1: see the board

1. Before you write any JSX, what are the three states this screen can be in before it shows real
   rows? What does the table look like in each one, and how would a test tell them apart?
2. Course lesson `113-live-coding-drills/01-how-a-screen-is-graded` names loading, empty, and error
   states as the first thing a grader checks for, precisely because they're the first thing a
   solo implementer forgets under time pressure.
3. Fetch the program's tasks once on mount (from the program id in the URL), and hold three
   possibilities in state: still loading, loaded with zero tasks, loaded with some tasks. Render a
   different thing for each rather than trying to infer "empty" from "an array that happens to be
   empty and also happens to still be null while loading," which is the bug this story is
   designed to catch.

## S3.2: find tasks fast

1. Does the filter need its own piece of state, separate from the full list of tasks? What would
   go wrong if you filtered by mutating the original list in place?
2. Course lesson `113-live-coding-drills/04-drill-task-table` is this exact filter-and-sort table,
   worked as a drill; lesson `54-react-performance/01-measuring-react-the-right-way` is worth
   reading before you decide whether you need `useDeferredValue` at this data size.
3. Keep the full task list as the source of truth, keep the filter text as its own state, and
   derive the visible rows (filtered, then sorted) on every render rather than storing a separate
   "filtered tasks" state that you'd have to remember to keep in sync. Track which column is
   sorted and in which direction as two small pieces of state (or one combined one), and reflect
   the current direction on the active header's `aria-sort` attribute.

## S3.3: flip status honestly

1. When the status control fires, what's the very first thing that should change, before any
   network call is even made? What has to happen afterward if that network call fails?
2. Course lesson `03-actions-and-optimistic-ui/03-optimistic-ui` covers the general shape; lesson
   `113-live-coding-drills/06-drill-optimistic-status` is this exact toggle, worked as a drill.
3. On a status change, update the task's status in your local state immediately, then send the
   `PATCH` request. On success, do nothing further (the state's already right). On failure, set
   the task's status back to what it was before the change, and set a separate piece of state
   holding a human-readable message, rendered inside an element with `aria-live="polite"` so it's
   announced whether or not anyone happened to be looking at the row that changed.

## S3.4: manage dependencies safely

1. What does the API tell you when a dependency would create a cycle, and where does that
   information need to end up so it's visible next to the specific control the user just used?
2. This story's failure mode (a request that gets rejected, with a reason the user needs to see
   right where they were working) is the same shape as S3.3's rollback; lesson
   `03-actions-and-optimistic-ui/03-optimistic-ui` is worth rereading with this story in mind
   instead of the status toggle.
3. Keep a small piece of state per task (or keyed by task id) for "the last dependency error for
   this task's control," separate from the task data itself. When the `POST` to add a dependency
   comes back with a `cycle` error, set that state to the message and render it inline near the
   control it belongs to; when the request succeeds, clear it and refresh that task's dependency
   list from the response.

## S3.5: know what's ready

1. What does "ready" depend on: something the API can tell you outright, or something you'd have
   to compute yourself from a task's status and its dependencies' statuses? Which is more likely
   to agree with the backend's own definition instead of quietly drifting from it?
2. The phase 2 contract already exposes a ready list endpoint; think about what it would cost you,
   correctness-wise, to reimplement that logic client-side from the raw task and dependency data
   instead of asking the API the same question it already knows how to answer.
3. Fetch the program's ready tasks the same way you fetch its full task list, and use that result
   to decide which rows get the badge, rather than recomputing "todo and every predecessor done"
   yourself from the dependency graph; that keeps the UI's definition of ready identical to the
   API's by construction instead of by careful copying.

## S3.6: respect vendor visibility

1. If the API already leaves `internalNote` out of the JSON for a caller who shouldn't see it,
   what does the UI actually need to build for this story?
2. There's no course pointer specific to this one; it's a reminder that the right amount of UI
   work for a security boundary the backend already enforces is sometimes none at all, and adding
   a client-side check on top of an already-absent field is often just more code with the same
   number of bugs it could have.
3. Render whatever the task object gives you for its note, the same way you'd render an optional
   due date. When the field isn't there, there's nothing to render, and that's the entire fix; if
   you find yourself writing a role check to decide whether to show it, that's a sign you're
   solving a problem the API already solved.

## S3.7: it works with a keyboard

1. Try to complete a full pass over the board (filter, sort a column, change a status, add a
   dependency) using only `Tab`, `Enter`, and `Space`, with your mouse physically out of reach. Where
   does it break?
2. Course lesson `58-keyboard-and-focus/01-keyboard-first` is about exactly this habit: building
   and testing the keyboard path first rather than retrofitting it onto a mouse-first screen.
3. This story mostly falls out for free if S3.1 through S3.4 are built from real interactive
   elements (`button`, `select`, labelled `input`) rather than styled `div`s with click handlers;
   the one place that usually needs deliberate attention is a sortable header, which needs to be a
   real button (or a `button` inside the header cell) so `Enter` and `Space` trigger it the same
   way a click would.

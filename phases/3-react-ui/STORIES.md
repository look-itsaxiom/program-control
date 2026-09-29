# Phase 3: React UI

Read the [primer](./PRIMER.md) first. This phase builds the board a program manager, a vendor
engineer, or a stranger actually looks at: a table of a program's tasks, filterable and sortable,
with a status control that feels instant, a dependency editor that refuses to let you create a
cycle, and a badge for what's ready to start. It talks to the phase 2 API you already built (or,
in the acceptance tests, a fake that speaks the same contract) and nothing else; there is no new
backend work in this phase.

## S3.1: see the board

As a program manager, I need to open a program and see its tasks as a table, so that I have one
place to look instead of guessing at status from a chat thread.

**Acceptance:** opening a program shows a table of its tasks with columns for title, status,
assignee, and due date. While the tasks are being fetched, the screen shows a loading state
instead of an empty or broken-looking table. A program with no tasks shows an empty state, not a
table that looks like it's still loading or like something went wrong.

**Contract:**

- The app's default export is `App`, from `web/src/App.tsx`; it renders whatever board matches
  the current URL.
- The program shown is read from the URL path `/programs/{programId}` (Chase may use any router,
  or none, to get from that path to a program id).
- Every API call goes to a relative URL (`/api/...`), proxied to the API in dev and replaced by a
  fake implementing the same contract in tests.
- The table carries `role="table"` and has column headers with the accessible names `Title`,
  `Status`, `Assignee`, `Due`.

## S3.2: find tasks fast

As a program manager looking at a program with sixty tasks, I need to narrow the table by typing
a few letters and to sort it by clicking a column, so that I can find the one task I'm looking for
without scrolling and squinting.

**Acceptance:** typing into a filter field narrows the visible rows to tasks whose title matches,
without a full-page reload or a visible flash of empty content while the match happens. Clicking a
column header sorts the table by that column; clicking it again reverses the order; the header
communicates which way it's currently sorted, not just to someone looking at it, but to someone
using a screen reader too.

**Contract:**

- A text input labelled `Filter tasks` narrows the visible rows.
- Column headers are clickable and toggle `aria-sort` (`ascending` / `descending`) on the header
  currently controlling sort order.

## S3.3: flip status honestly

As a program manager, I need to flip a task's status and see it change immediately, and see it
flip back with a message if the save fails, so that the board feels live without ever lying to me
about what's actually saved.

**Acceptance:** changing a task's status is reflected on the board right away, before the network
request that saves it has finished. If that save succeeds, nothing more happens; the board was
already showing the truth. If it fails, the status reverts to what it was before, and a message
saying so is available to everyone looking at the screen, including someone using a screen reader,
not only someone who happened to be looking at the exact pixel that changed.

**Contract:**

- Each row has a status control (a `<select>` or a set of buttons) whose accessible name is the
  task's title.
  If it is a set of buttons, the button for the current status carries `aria-pressed="true"`
  and the others `aria-pressed="false"`, so the current status can be read without guessing.
- An `aria-live="polite"` region carries the failure message when a status change is rolled back.

## S3.4: manage dependencies safely

As a program manager, I need to add a dependency between two tasks and be told plainly when that
would create a cycle, so that I can wire up a real project plan without silently corrupting it.

**Acceptance:** adding a dependency to a task offers the other tasks in the program to choose
from. Choosing one that would make the dependency graph circular (directly, or through a chain of
other dependencies) is refused, with an explanation that says, in terms a non-programmer would
understand, that it's a cycle; the dependency is not added, and the rest of the board is
undisturbed.

**Contract:**

- Each task has a dependency control: a `<select>` labelled `Add dependency to <title>` (with the
  task's actual title in place of `<title>`).
- A rejected addition shows an inline error, near that control, whose text contains the word
  `cycle`.

## S3.5: know what's ready

As a vendor engineer, I need to see at a glance which tasks are actually ready to start, so that I
don't waste time picking up something that's still blocked on someone else.

**Acceptance:** a task shows a visible "Ready" indicator exactly when it is startable right now:
its own status is not yet started, and every task it depends on is finished. A task with nothing
left to wait on (no dependencies at all) can still be ready. A task marked as blocked never shows
as ready, even if nothing it depends on is holding it up.

**Contract:**

- A "Ready" badge appears on a task's row exactly when the task is ready, by the rule above.

## S3.6: respect vendor visibility

As a customer program manager, I need the internal note I leave on a task to stay inside my own
organization, so that a vendor looking at the same board never sees something I wrote for my own
team.

**Acceptance:** a caller outside a task's owning organization never sees that task's internal note
anywhere on the board, in any form (not truncated, not behind a click, not in a tooltip): the data
simply never reaches them, because the API already leaves it out of the response for them.

**Contract:** none beyond phase 2's rule that `internalNote` is omitted, not nulled, for a caller
outside the task's owning organization; the UI has nothing further to build for this story beyond
not inventing a way to show a field it was never given.

## S3.7: it works with a keyboard

As a program manager who does not want to touch a mouse, I need to reach every control on the
board (the filter, the sortable headers, a task's status control, its dependency control) using
only the keyboard, in an order that makes sense, so that the board is usable at all for someone
who can't or won't point and click.

**Acceptance:** every interactive element on the board (filter input, sortable column headers,
each row's status control, each row's dependency control) can be reached with `Tab` alone, in a
sensible order, and operated without a pointer (a header's sort can be triggered with `Enter` or
`Space`, a `<select>` opened and changed with the keyboard as browsers already support). Nothing
on the board is a `div` or `span` pretending to be interactive without the keyboard support that
comes for free with a real button, link, or form control.

**Contract:** none beyond the DOM already named in S3.1 through S3.4; this story is about how
those elements behave, not about any additional element.

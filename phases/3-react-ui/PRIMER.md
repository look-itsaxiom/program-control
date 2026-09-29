# Primer: React UI

## Start from the data, not from the boxes

It's tempting to open this phase by sketching components: a `TaskTable`, a `TaskRow`, a
`StatusSelect`. Resist that for the first hour. Start instead from the data you have (a list of
tasks, each with dependencies) and the questions the screen has to answer (which tasks are
visible right now, in what order, and what does each one's status control currently say). Once
you know what state actually needs to exist and who's allowed to change it, the component
boundaries tend to fall out on their own, in the places where the data naturally splits. Course
lesson `113-live-coding-drills`, section `01-how-a-screen-is-graded`, is written for exactly this
situation: a take-home or live-coding UI task judged on what a screen does under pressure, not on
how many components it has.

## Where does each piece of state actually live?

Not every piece of state belongs in one place. The full list of tasks belongs somewhere that can
be shared by the table, the filter, and the sort control. The filter's current text and the sort
column probably belong closer to the table itself, since nothing else needs them. Whether a
specific row is mid-save (for the optimistic update in S3.3) is arguably that row's own business,
not the whole table's. Getting this placement right is less about a rule and more about asking,
for each piece of state, "what's the smallest set of components that need to read or write this?"
and putting it there. Lesson `113-live-coding-drills`, drill `04-drill-task-table`, is a close
cousin of this phase's table and is worth working through directly.

## Optimistic UI, and what "roll back" really means

An optimistic update shows the new state before the server has confirmed it, then either does
nothing further (success) or reverts (failure) with a visible explanation. The hard part isn't the
happy path, it's making the rollback trustworthy: if two status changes on the same task race each
other, or if the network is slow enough that a user clicks twice, does the board end up showing
what's actually saved, or does it end up out of sync in a way nobody can see? Course lesson
`03-actions-and-optimistic-ui`, section `03-optimistic-ui`, covers the shape of this pattern in
general; lesson `113-live-coding-drills`, drill `06-drill-optimistic-status`, is the same status
toggle this story asks for, worked as a drill.

## Deferred values for a filter that doesn't stutter

A filter wired directly to every keystroke can feel fine with sixty rows and feel awful with six
hundred, because every keystroke forces a full re-render of a big list while the user is still
typing. React's tools for deferring a value (letting the input feel instant while the filtered
list catches up a moment later) exist for exactly this. Whether you need them at this phase's data
size is a fair question to ask yourself; the point of this section is knowing the tool exists and
what problem it solves, so the choice is deliberate either way. Course lesson `54-react-performance`
covers this class of problem; section `01-measuring-react-the-right-way` is worth reading before
you reach for the fix, so you're solving a measured problem instead of an imagined one.

## Keyboard and ARIA basics, not as an afterthought

A table with a filter, sortable headers, a status control per row, and a dependency control per
row has a lot of interactive surface. Building it with real, semantic, focusable elements (a
`button`, a `select`, an `input`) gets you most of keyboard support for free; building it with
styled `div`s gets you none of it, and retrofitting keyboard support onto a div-based
control after the fact is far more work than starting with the right element. `aria-sort` on a
sortable header and an `aria-live` region for the rollback message aren't decoration; they're the
only way a screen reader user finds out those two things happened at all. Course lesson
`58-keyboard-and-focus`, sections `01-keyboard-first` and `03-managing-focus-deliberately`, covers
both halves of this directly.

## Tailwind as a system, not a pile of classes

Utility classes are easy to reach for one at a time and easy to end up with an inconsistent screen
from, if every component invents its own spacing and color choices. Treating Tailwind's scale
(spacing, color, type) as a small design system you draw from consistently, rather than as free
text, is what keeps a table, a filter, and a badge looking like they belong to the same product.
Course lesson `18-styling-in-2026`, section `03-component-styling-patterns`, covers this habit
directly, including when a repeated utility cluster is worth pulling into its own component versus
leaving inline.

## The interview angle

A live-coding UI exercise is usually judged on whether the obvious-looking table actually holds up
under the follow-up questions: what happens while it's loading, what happens when it's empty, what
happens when the save fails, can you drive it without a mouse. Those are the four questions this
phase's stories ask directly, on purpose.

## Questions to answer for yourself

1. Where does "this task's dependency-cycle error message" live? Is it state on the task, state on
   the dependency control, or something more local, and what's the smallest set of components that
   need to know about it while it's showing?
2. If a status save is in flight and the same status control fires again before it resolves, what
   should happen? You don't have to solve this for the acceptance tests, but it's worth having an
   answer before you're surprised by it.
3. What's the actual difference, for a screen reader user, between text that changes color and
   text that changes inside an `aria-live` region? Why does the rollback message need the latter?

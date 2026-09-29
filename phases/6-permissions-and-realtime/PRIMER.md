**STRETCH.** This phase is optional, and builds on phase 5, itself optional. Skip both if you're
short on time; nothing in phases 0 to 4 depends on either.

# Primer: permissions and realtime

## Visibility is a query, not a filter bolted on after

Phase 2's tenant rule (S2.8) was binary: a caller is either in the program's owning or shared
organization, or they aren't, and the answer to "can they see this program at all" doesn't change
per task. This phase adds a second, finer-grained rule on top of that: even a caller who can see
the program at all shouldn't see every task in it at full detail, only the ones their own
organization owns, plus just enough about the tasks those depend on to understand why their own
work is or isn't ready to start. The temptation is to fetch everything and strip fields in the
response-serialization step, after the database has already done the work of finding rows the
caller was never going to see. The better habit is to let the visibility rule shape the query
itself, the same way S1.8 shaped an index around the query the application actually runs. Course
lesson `112-system-design-cross-org-collaboration`, section
`01-visibility-as-a-first-class-query`, covers this distinction.

## Partial objects are still objects with a contract

"Show only `id`, `title`, and `status`" is not the same rule as "show the full object with the
other fields blanked out." A blanked-out field (`estimateMinutes: null`) still tells a vendor that
an estimate exists and was hidden; an absent key tells them nothing was ever disclosed about
whether one exists at all. This project has already drawn that line once, in S2.8's rule about
`internalNote`; this phase draws it again, more broadly, for an entire class of tasks a vendor is
only allowed to know exist and have a name and a status. Keeping that distinction sharp in your
response-shaping code (a different struct, or a different serialization path, rather than the same
struct with fields left at their zero value) is what keeps "hidden" and "absent" from quietly
becoming the same bug six months from now.

## An audit trail is a promise about the past, not the present

`GET .../tasks` answers "what is true right now." An events endpoint answers a different question:
"what happened, in order, and who did it," and that answer has to stay true even after the present
has moved on (a task's current status no longer tells you it was briefly `blocked` last Tuesday).
That's why S1.5 gave `task_events` its own auto-incrementing id instead of relying on a timestamp
to order it: two events in the same millisecond still need an unambiguous order, and a caller
paging through history with `since=<id>` needs a cursor that can never repeat or go backward, which
a timestamp alone does not always guarantee.

## Long-lived connections are a different contract than request/response

Everything through phase 5 is one request in, one response out, done. Server-sent events keep a
single HTTP response open indefinitely and write to it repeatedly as things happen, which changes
several assumptions a request handler normally gets to make for free: the handler has to know when
the client goes away (so it stops trying to write to a connection nobody is reading), and something
elsewhere in the server has to have a way to reach a specific open connection when the event it
cares about happens, which a normal handler, which only ever knows about the one request it's
currently serving, does not automatically have. Course lesson
`112-system-design-cross-org-collaboration`, section `02-long-lived-connections`, covers the
broadcast/subscriber shape this usually takes: something that fires an event, and a registry of
currently-open connections it fans that event out to.

## The interview angle

"Add row-level permissions to an existing list endpoint" and "make one part of the UI update live"
are both extremely common follow-up questions to a take-home like this one, precisely because they
test whether your first design already has a seam to extend, or whether the fix has to be bolted on
sideways. If your phase 2 query already took a caller's identity as a parameter, extending it with a
finer rule here should feel like widening that parameter's job, not restructuring the query from
scratch.

## Questions to answer for yourself

1. If a vendor asks for a task by id directly (not through the list endpoint) and that task is a
   predecessor of one they own, should the answer differ from what the list endpoint would have
   shown them for that same task? Why or why not?
2. What would break for a client using `since=<eventId>` to page through history if two different
   database transactions could occasionally hand out the same event id?
3. If the connection carrying an SSE stream to one browser tab dies without a clean close (the
   laptop goes to sleep), what does your server need to notice, and what happens if it never does?

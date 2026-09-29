**STRETCH.** This phase is optional. Program Control's core loop (phases 0 to 4) stands on its
own; this phase and the two after it push into territory a take-home rarely asks for outright, but
a real job asks for constantly.

# Primer: integrations

## Your server is not the only caller anymore

Every phase up to this one assumed a request comes from a person, identified by a header a
browser or a test sent. A webhook breaks that assumption: the caller is another company's
computer, running unattended, with no person behind it to notice or retry a failure. That changes
what "authenticate the request" means. There is no `X-User-Id` to trust, because there is no user;
there is a shared secret, and the request has to prove it knows that secret without the secret
ever traveling over the wire in a form an eavesdropper could steal and replay. There is also no
browser tab to show a spinner in if your handler is slow; the caller's retry policy, not your UI,
decides what happens if you take too long. Course lesson `105-integrations-in-go`, section
`01-webhooks-as-untrusted-input`, is about treating an inbound webhook as hostile until proven
otherwise, which is the mindset the rest of this primer builds on.

## Proving a message wasn't tampered with, and wasn't replayed

An HMAC signature answers one question: "did the holder of this secret produce this exact byte
string?" It does not answer "was this message sent five minutes ago or five days ago?" and it does
not answer "have I already acted on this exact message once?" Those are two separate problems
that a signature alone does not solve, and a real integration has to solve both: a timestamp
outside a tolerance window catches a captured request being replayed long after the fact, and an
idempotency check (remembering which message ids you've already handled) catches the much more
common case of a caller's own retry logic sending the same message twice because it never saw your
first response. Course lesson `105-integrations-in-go`, section `02-verifying-signed-webhooks`,
covers the signature half; think about why the timestamp check and the idempotency check are both
still necessary even with a correct signature.

## Rotating a secret without an outage

A shared secret eventually needs to change, whether because a vendor asks for it or because
your own security policy requires it on a schedule. The naive approach, swap the secret and
redeploy, has a window where the vendor is still signing with the old value and your server has
already stopped accepting it, and every webhook sent in that window is silently rejected. Accepting
two secrets at once, a current one and a previous one, during a rotation window turns that outage
into a non-event: you change the secret whenever you want, tell the vendor whenever they get to
it, and only remove the previous value once you're confident nothing is still using it. Course
lesson `105-integrations-in-go`, section `03-rotating-shared-secrets`, walks through why this
two-value window is the standard pattern, not a workaround.

## Don't make the caller wait for your side effects

A webhook handler that updates a task's status and then, in the same request, calls out to a
third system (a chat notification, an email, another vendor's API) has coupled two things that
don't need to be coupled: the caller's confidence that you received the message, and the success
of whatever you do next with it. If that third call is slow or down, the caller's request hangs or
times out, even though the part they actually asked you to do (accept the webhook) already
succeeded. Doing the side effect after the response has already gone out, off the request's own
goroutine, keeps those two concerns separate: the response answers "did you get this," and
whatever happens next answers a different question the caller was never asking. Course lesson
`105-integrations-in-go`, section `04-background-work-and-retries`, covers this handoff and the
retry-with-backoff pattern for the part that runs after the response.

## The interview angle

"Design a webhook receiver" is a common systems question precisely because it's small enough to
finish in an interview and still surfaces real judgment: how do you verify the sender, how do you
handle the same message arriving twice, what happens when your downstream call fails, and does any
of that failure ever become the caller's problem. Being able to name each of those four questions
unprompted, and explain the trade-off in each answer, is usually worth more than the code itself.

## Questions to answer for yourself

1. If you checked the signature after checking whether the message id was already seen, instead of
   before, what could an attacker with no valid secret still make your server do?
2. Why does accepting two secrets at once during rotation not weaken security the way accepting
   "any of several passwords" would for a login system?
3. What's the actual difference, in what the caller experiences, between "the notification failed
   and nobody knows" and "the notification failed and it will be retried in the background"? Which
   one does a queue and a worker actually buy you?

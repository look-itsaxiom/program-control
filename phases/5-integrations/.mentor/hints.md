# Hints: phase 5 (stretch)

Hidden from the learner. Read only by the mentor. Give one rung at a time, only when asked, and
only after the learner has tried the step themselves. Rung 3 describes the shape of a solution in
prose; it never contains code, and it should still leave the learner writing the Go.

## S5.1: accept a signed vendor webhook

1. The signature covers `"<timestamp>.<body>"`. What has to happen to the request body before you
   can both verify the signature against it and unmarshal it as JSON? (Hint: you can only read an
   `http.Request.Body` once.)
2. Course lesson `105-integrations-in-go/01-receiving-webhooks-you-can-trust` covers computing and
   comparing an HMAC in Go, including the constant-time comparison you want instead of `==` on the
   hex strings (timing attacks on signature comparison are a real, named category of bug). The same
   section covers accepting two secrets during a rotation window, and the exercise
   `105-integrations-in-go/02-webhook-receiver` has the learner build the verifier end to end.
3. Read the whole body into a `[]byte` first with `io.ReadAll`, compute your own HMAC over
   `timestamp + "." + string(body)` using `crypto/hmac` and `crypto/sha256`, and compare it against
   the decoded `X-Signature` with `hmac.Equal` (not `==`), trying `WEBHOOK_SECRET` and, if set,
   `WEBHOOK_SECRET_PREVIOUS`. Only after a signature matches do you parse the timestamp, compare it
   to `time.Now()`, and only after that do you check an in-memory or database-backed set of
   already-seen message ids for the duplicate case.

## S5.2: notify downstream without blocking

1. What Go primitive lets one goroutine hand work to another without the first goroutine waiting
   for the second to finish?
2. Course lesson `105-integrations-in-go/03-calling-out-clients-that-survive-outages` covers exactly
   this handoff: a buffered channel a worker goroutine reads from, and a small retry loop with
   `time.Sleep` growing between attempts (or `time.After` in a `select`, so a slow attempt doesn't
   block the retry loop's own bookkeeping forever).
3. After a status change is durable, construct the `Event` payload and send it on a channel instead
   of calling the downstream `POST` inline; a worker goroutine, started once when the server boots,
   reads from that channel and does the actual HTTP call, retrying up to 3 times with a backoff
   (even something as simple as doubling a starting delay is enough) before giving up on that one
   delivery and moving to the next item.

## S5.3: background work

1. What would happen to a request handler that pushed onto an unbuffered channel with nothing ever
   reading from it? What does that imply about the channel's buffer size, or about starting the
   worker before the first request can arrive?
2. Course lesson `105-integrations-in-go/03-calling-out-clients-that-survive-outages`, again, with
   its exercise `04-resilient-client`: sizing a channel's buffer and deciding what to do when it is
   full (drop, block, or grow) is a judgment call, and any of the three is a defensible choice for a
   take-home as long as you can say which one you made and why.
3. There's no new shape here beyond S5.2's worker; this story is about making sure that worker is
   started once, outside any single request's lifetime, and that the request handler's own path
   never itself performs the downstream HTTP call.

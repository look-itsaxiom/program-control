# Three-minute demo

Run `pnpm db:reset && pnpm seed && pnpm dev` first. The seed prints the program id and the four
user ids; keep that output on screen. Sign in by setting `localStorage['programControl.userId']`
in the browser console, then reload.

## 1. The board, as the customer (about 40 seconds)

Sign in as Alice and open `/programs/<programId>`.

- The table lists every task in the campaign, ordered by due date with undated work last.
- "Draft engine spec" is done, so "Machine the housing" and "Cast the manifold" carry a **Ready**
  badge: their only predecessor is finished. "Assemble and test" does not, because it is still
  waiting on both of them.
- "Certify for export" is blocked and never shows Ready, however its predecessors look. A human
  has already said it is not workable.
- Type `manifold` into the filter. The table narrows without a reload. Click the **Title** header
  twice and watch `aria-sort` flip; a screen reader hears the change, not just the arrow.

## 2. Optimistic status, honestly (about 30 seconds)

Change "Cast the manifold" to Doing. The row updates before the save comes back.

Now stop the API (`Ctrl+C` in the `pnpm dev` terminal) and change it again. The row snaps back to
where it was, and the live region announces why. The board never shows something that is not
saved. Restart the API before continuing.

## 3. Cycles are refused, in plain words (about 25 seconds)

On "Draft engine spec", open the dependency control and pick "Requires Assemble and test".

"Assemble and test" already waits on the spec through two other tasks, so this would close a loop.
The API answers `409` with `error.code: "cycle"`, and the row shows an explanation that says so
without using the word "graph". Nothing was added.

## 4. What a vendor sees (about 45 seconds)

Sign in as Bob (Halden Machine Works) and reload the same URL.

- He sees his own two tasks in full.
- He sees "Draft engine spec" as three fields only: id, title, status. He needs to know why he was
  waiting, not what the customer wrote about margin.
- The customer's "Assemble and test", "Dyno run at full load", and "Certify for export" are not
  hidden behind a lock icon. They are simply not in the response.
- Fetch one of them directly: `GET /tasks/<id>` with Bob's header answers `404`.

## 5. It stays honest without a refresh (about 30 seconds)

Leave Bob's board open. In another terminal, send the vendor webhook for "Machine the housing":

```sh
curl -sS -X POST localhost:8080/webhooks/vendor \
  -H 'Content-Type: application/json' \
  -H "X-Timestamp: $(date +%s)" \
  -H "X-Signature: <hex hmac of '<timestamp>.<body>'>" \
  -d '{"id":"demo-1","type":"task.status","taskId":"<housing id>","status":"done"}'
```

Bob's row changes on screen, pushed over the stream, with no request from the browser. Send the
same message again and the API answers `{"status":"duplicate"}` and changes nothing.

Finish on `GET /programs/<programId>/events`: the whole history, in order, including that webhook,
with no actor because no person performed it.

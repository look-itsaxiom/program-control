# Primer: REST API

## A handler is a function, and a server is a tree of them

Go doesn't make you learn a framework to answer an HTTP request; `net/http` builds routing,
middleware, and JSON on top of one interface with a single method. A handler is, in practice,
just a function with the signature `func(http.ResponseWriter, *http.Request)`, and
`http.ServeMux` since Go 1.22 matches both method and path pattern (`GET /tasks/{id}`), so you get
the router-library shape without a router library. Read this phase's routes in the stories as a
list of `(method, path) -> handler` pairs before you write a line of code; deciding that list is a
design step, not a mechanical one, because it's the last time you'll see the whole surface area at
once. Course lesson 103, section "01-a-server-is-a-function," walks through the handler
abstraction and the mux's pattern syntax directly.

## Middleware is one function type, and order is the design

`func(http.Handler) http.Handler`: take a handler, return a handler that wraps it. Every
cross-cutting concern in this phase (who's calling, what request id this is, turning a panic into
a clean response) fits that one shape, composed into a chain around your routes rather than
copy-pasted into every handler. The chain's order is not cosmetic: something that reads a value
from the request's context has to run after whatever middleware put it there, and something
meant to catch every failure has to sit close to the outside of the chain or a failure inside it
slips past uncaught. Course lesson 103, section "03-middleware-and-structure," builds this pattern
from scratch, including the request-id and auth-style middleware this phase needs a version of.

## JSON discipline: read carefully, write consistently

Decoding a request body and encoding a response body are both places where a small, consistent
habit pays for itself later. On the way in: cap the body size before you decode it, and decide on
purpose whether an unrecognized field is an error or silently ignored (this phase's stories don't
require rejecting unknown fields, but knowing that `json.Decoder.DisallowUnknownFields` exists is
worth having). On the way out: set headers and call `WriteHeader` before writing any body bytes,
since the first write locks in an implicit `200` if you haven't set a status yet. Both directions
also mean picking one shape for "a list of things" (an empty JSON array, never `null`, when
there's nothing to return) and one shape for "something went wrong" (the error envelope this
phase's stories fix), and using it everywhere rather than reinventing it per route.

## The error envelope as a contract, not a convenience

Every error in this phase, no matter which story or which failure inside it, looks the same:
`{"error":{"code":"...","message":"..."}}`. That uniformity is what lets a client (or, in phase 3,
your own frontend) branch on `error.code` once, generically, instead of parsing each route's
error message by hand. It also means the mapping from an internal failure (a constraint violation
from phase 1's schema, a validation failure, a missing row) to an HTTP status and a `code` string
happens in one place, at the edge, not scattered across every handler that happens to touch the
database.

## `context.Context`: deadlines, cancellation, and request-scoped values

Every `*http.Request` carries a `context.Context`, reachable with `r.Context()`, and it's the
right home for exactly two kinds of thing: a deadline or cancellation signal, and a small bag of
request-scoped values that many layers might want to read (a request id, the caller identified in
S2.1) without threading them through every function's parameter list by hand. It is not the right
home for anything a function's behavior actually depends on; that belongs in the function's real
parameters, where a missing value is a compiler error, not a runtime surprise. Course lesson 103's
section on context, and lesson 104, section "01-shaping-a-service," on error-handling patterns
(sentinel errors, typed errors, and mapping a domain error to a transport code once, at the edge)
both apply directly to how this phase turns "the database refused that insert" into the right
`error.code`.

## Testing an HTTP handler without a running server

`httptest.NewRequest` and `httptest.NewRecorder` let a test call a handler's `ServeHTTP` directly,
in-process, with no listening socket and no network round trip: build a request, hand it to the
handler, and inspect the recorder's status, headers, and body. That's what every test in
`api/phase_tests` does, and it's also the fastest way to iterate on a handler yourself while
you build it, well before you'd reach for a real client against a real server.

## Why 404, not 403, across a tenant boundary

A `403 Forbidden` tells the caller "I know what's here, and you can't have it," which is itself a
disclosure: a stranger probing a program id learns that id is real. A `404 Not Found` makes
"exists, but you can't see it" and "doesn't exist at all" indistinguishable from the outside,
which is exactly the property a multi-tenant API needs at its boundary. This phase's stories
apply that rule at the program level (S2.8); the same instinct is worth carrying past this phase
into anything else that's shared, rather than owned, across an organization boundary.

## The interview angle

A REST API exercise like this one gets judged less on "did every route work" and more on whether
the error handling, the auth check, and the tenant boundary are all treated as first-class parts
of the design instead of afterthoughts bolted onto working happy-path handlers. Building the
error envelope and the auth middleware once, early, and reusing them everywhere, reads very
differently in a review than writing the happy path for every route first and retrofitting errors
in at the end.

## Questions to answer for yourself

1. Where does "the caller is identified" belong: something every handler checks for itself, or
   something one piece of middleware establishes once, before any handler runs? What would go
   wrong if it were the former and one handler forgot?
2. The cycle check in S2.5 has to reason about the whole dependency graph, not just the one new
   edge. What's the smallest piece of information you'd need to walk to decide whether a new edge
   closes a cycle, and where would you get it?
3. S2.8's `internalNote` omission depends on comparing the caller's organization to the task's
   owning organization on every single task in a list, not just once per request. Where does that
   comparison naturally belong: in the SQL, in the handler, or in whatever turns a database row
   into JSON?

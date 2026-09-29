// Package httpapi is the REST layer: routing, authentication, the shared JSON error envelope,
// and the translation between HTTP and the store's domain types. No SQL lives here.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5"

	"github.com/look-itsaxiom/program-control/api/internal/events"
	"github.com/look-itsaxiom/program-control/api/internal/notify"
	"github.com/look-itsaxiom/program-control/api/internal/store"
)

// Config configures the HTTP API.
type Config struct {
	DatabaseURL string
	Logger      *slog.Logger

	// QueryTracer, when set, is installed on every connection the handler opens (phase 4's
	// query-count contract depends on this: a test builds a counting tracer, passes it here,
	// and asserts on how many queries a GraphQL request issues).
	QueryTracer pgx.QueryTracer
}

// server holds everything the handlers share. There are no package-level variables: a test can
// build as many independent handlers as it likes.
type server struct {
	store    *store.Store
	log      *slog.Logger
	notifier *notify.Notifier
	broker   *events.Broker
	secrets  webhookSecrets
}

// publishChange is the one place a completed status change fans out: to the downstream notifier
// (phase 5) and to every open live stream (phase 6). Both are best-effort and neither blocks, so
// a caller can hand a change over after its own response has already been written.
func (s *server) publishChange(task store.Task, change store.StatusChange) {
	s.notifier.Notify(change)
	s.broker.Publish(task)
}

// NewHandler opens the store, wires the routes, and returns the API. The pool it owns is closed
// when ctx is done, so a caller that cancels its context leaks nothing.
func NewHandler(ctx context.Context, cfg Config) (http.Handler, error) {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}

	st, err := store.New(ctx, cfg.DatabaseURL, cfg.QueryTracer)
	if err != nil {
		return nil, err
	}
	go func() {
		<-ctx.Done()
		st.Close()
	}()

	notifier := notify.New(os.Getenv("WEBHOOK_NOTIFY_URL"), log)
	notifier.Start(ctx)

	s := &server{
		store:    st,
		log:      log,
		notifier: notifier,
		broker:   events.New(),
		secrets: webhookSecrets{
			current:  os.Getenv("WEBHOOK_SECRET"),
			previous: os.Getenv("WEBHOOK_SECRET_PREVIOUS"),
		},
	}
	return s.routes(), nil
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, s.log, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /programs/{programId}/tasks", s.authenticate(s.listTasks))
	mux.HandleFunc("POST /programs/{programId}/tasks", s.authenticate(s.createTask))
	mux.HandleFunc("GET /programs/{programId}/tasks/ready", s.authenticate(s.listReadyTasks))

	mux.HandleFunc("GET /tasks/{id}", s.authenticate(s.getTask))
	mux.HandleFunc("PATCH /tasks/{id}", s.authenticate(s.patchTask))

	mux.HandleFunc("POST /tasks/{id}/dependencies", s.authenticate(s.addDependency))
	mux.HandleFunc("DELETE /tasks/{id}/dependencies/{predecessorId}", s.authenticate(s.removeDependency))

	mux.HandleFunc("GET /programs/{programId}/events", s.authenticate(s.listEvents))
	mux.HandleFunc("GET /programs/{programId}/stream", allowQueryIdentity(s.authenticate(s.streamProgram)))

	mux.HandleFunc("POST /webhooks/vendor", s.receiveVendorWebhook)

	mux.Handle("POST /graphql", s.graphqlHandler())

	return withRequestID(s.recoverPanics(s.envelopeMuxErrors(mux)))
}

// envelopeMuxErrors rewrites the two responses ServeMux produces by itself, a 404 for a path no
// route claims and a 405 for a route called with a method it does not accept, into the same JSON
// envelope every handler here uses. It leaves a handler's own 404 alone, recognizing it by the
// Content-Type the handler has already set.
//
// Registering a catch-all "/" route instead would not work: it would match every method on every
// path, so ServeMux would never report a method mismatch and the 405 would be lost.
func (s *server) envelopeMuxErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		iw := &envelopingWriter{ResponseWriter: w}
		next.ServeHTTP(iw, r)

		switch iw.intercepted {
		case http.StatusNotFound:
			writeError(w, s.log, http.StatusNotFound, codeNotFound, "no route matches "+r.URL.Path)
		case http.StatusMethodNotAllowed:
			writeError(w, s.log, http.StatusMethodNotAllowed, codeMethod,
				r.Method+" is not accepted here; see the Allow header")
		}
	})
}

// envelopingWriter swallows a bare 404 or 405 and its plain-text body so the wrapper above can
// answer with the JSON envelope instead. Anything a handler wrote itself, which it marks by
// setting Content-Type, passes straight through.
type envelopingWriter struct {
	http.ResponseWriter
	intercepted int
}

func (w *envelopingWriter) WriteHeader(status int) {
	isMuxError := status == http.StatusNotFound || status == http.StatusMethodNotAllowed
	if isMuxError && w.Header().Get("Content-Type") != "application/json" {
		w.intercepted = status
		return
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *envelopingWriter) Write(b []byte) (int, error) {
	if w.intercepted != 0 {
		return len(b), nil
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap exposes the writer underneath so http.ResponseController can reach the capabilities a
// wrapper hides, flushing above all: without this the live stream could never push a byte.
func (w *envelopingWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// fail logs an unexpected error with its request id and answers 500 without echoing the error to
// the caller, who has no use for it and should not learn about the database from it.
func (s *server) fail(w http.ResponseWriter, r *http.Request, what string, err error) {
	s.log.Error(what,
		"error", err, "method", r.Method, "path", r.URL.Path,
		"request_id", requestIDFrom(r.Context()))
	writeError(w, s.log, http.StatusInternalServerError, codeInternal,
		"the server failed to handle this request")
}

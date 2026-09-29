package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/look-itsaxiom/program-control/api/internal/store"
)

const (
	headerRequestID = "X-Request-Id"
	headerUserID    = "X-User-Id"
)

type contextKey int

const (
	requestIDKey contextKey = iota
	userKey
)

// requestIDFrom returns the id assigned to this request, or "" outside the middleware.
func requestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// userFrom returns the authenticated caller. It is only ever called from a handler behind
// authenticate, which guarantees the value is there.
func userFrom(ctx context.Context) store.User {
	u, _ := ctx.Value(userKey).(store.User)
	return u
}

// withRequestID gives every request an id, echoing the caller's when they sent one and inventing
// one otherwise, and puts it on the response so a client can quote it in a bug report.
func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(headerRequestID)
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set(headerRequestID, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

// recoverPanics turns a panic in a handler into a 500 with the shared envelope, so one bad
// request cannot take the process down.
func (s *server) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				s.log.Error("panic serving request",
					"panic", v, "method", r.Method, "path", r.URL.Path,
					"request_id", requestIDFrom(r.Context()))
				writeError(w, s.log, http.StatusInternalServerError, codeInternal,
					"the server failed to handle this request")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// authenticate resolves X-User-Id into the calling user and their organizations. There is no
// session or token here: the header is the whole authentication story, standing in for a login
// system a later phase would add.
func (s *server) authenticate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get(headerUserID)
		if raw == "" {
			writeError(w, s.log, http.StatusUnauthorized, codeUnauthenticated,
				"this route requires an "+headerUserID+" header naming a known user")
			return
		}
		if _, err := uuid.Parse(raw); err != nil {
			writeError(w, s.log, http.StatusUnauthorized, codeUnauthenticated,
				headerUserID+" is not a user id")
			return
		}

		user, err := s.store.FindUser(r.Context(), raw)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, s.log, http.StatusUnauthorized, codeUnauthenticated,
					headerUserID+" does not name a known user")
				return
			}
			s.fail(w, r, "look up caller", err)
			return
		}

		next(w, r.WithContext(context.WithValue(r.Context(), userKey, user)))
	}
}

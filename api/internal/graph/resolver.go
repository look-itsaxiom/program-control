// Package graph is the GraphQL layer over the same store the REST API uses. Resolvers here hold
// no state of their own beyond the store; everything scoped to one request, the caller and the
// loaders, travels in the request context.
package graph

import (
	"context"
	"log/slog"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/look-itsaxiom/program-control/api/internal/store"
)

// Resolver is the root resolver, wired to the store.
type Resolver struct {
	Store *store.Store
	Log   *slog.Logger
}

// NewResolver returns a root resolver backed by st.
func NewResolver(st *store.Store, log *slog.Logger) *Resolver {
	return &Resolver{Store: st, Log: log}
}

// Error codes this layer puts in extensions.code. They are the stable, machine-checkable part of
// a failure; the message is for a person.
const (
	CodeUnauthenticated = "UNAUTHENTICATED"
	CodeNotFound        = "NOT_FOUND"
	CodeCycle           = "CYCLE"
	CodeValidation      = "VALIDATION"
	CodeInternal        = "INTERNAL"
)

type contextKey int

const (
	userKey contextKey = iota
	loadersKey
)

// WithUser puts the authenticated caller on the context. The HTTP layer does this before handing
// a request to the graph, so a resolver never reads a header.
func WithUser(ctx context.Context, user store.User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

// WithLoaders puts a fresh set of per-request loaders on the context.
func WithLoaders(ctx context.Context, loaders *Loaders) context.Context {
	return context.WithValue(ctx, loadersKey, loaders)
}

// callerFrom returns the authenticated caller, or an UNAUTHENTICATED error when the request never
// carried one.
func callerFrom(ctx context.Context) (store.User, error) {
	user, ok := ctx.Value(userKey).(store.User)
	if !ok {
		return store.User{}, Errorf(ctx, CodeUnauthenticated, "this request is not signed in")
	}
	return user, nil
}

func loadersFrom(ctx context.Context) *Loaders {
	loaders, _ := ctx.Value(loadersKey).(*Loaders)
	return loaders
}

// Errorf builds a GraphQL error carrying a stable extensions.code and the field path it happened
// at, which is what a client branches on.
func Errorf(ctx context.Context, code, format string, args ...any) *gqlerror.Error {
	err := gqlerror.Errorf(format, args...)
	err.Path = graphql.GetPath(ctx)
	err.Extensions = map[string]any{"code": code}
	return err
}

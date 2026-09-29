package httpapi

import (
	"errors"
	"net/http"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/google/uuid"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/look-itsaxiom/program-control/api/internal/graph"
	"github.com/look-itsaxiom/program-control/api/internal/graph/generated"
	"github.com/look-itsaxiom/program-control/api/internal/store"
)

// graphqlHandler builds the /graphql endpoint: the same store, the same identity header, and a
// fresh set of loaders per request so nothing is ever cached across callers or past a write.
func (s *server) graphqlHandler() http.Handler {
	srv := handler.New(generated.NewExecutableSchema(generated.Config{
		Resolvers: graph.NewResolver(s.store, s.log, s.notifier),
	}))
	srv.AddTransport(transport.POST{})
	srv.SetQueryCache(lru.New[*ast.QueryDocument](100))
	srv.Use(extension.Introspection{})

	return s.graphqlAuth(srv)
}

// graphqlAuth applies S2.1's identity rule to the graph. A failure here is expressed the way
// GraphQL expresses failure, an errors array carrying extensions.code, rather than as a bare HTTP
// status a GraphQL client would not know how to read.
func (s *server) graphqlAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get(headerUserID)
		if raw == "" {
			s.writeGraphQLAuthError(w, "this request carries no "+headerUserID+" header")
			return
		}
		if _, err := uuid.Parse(raw); err != nil {
			s.writeGraphQLAuthError(w, headerUserID+" is not a user id")
			return
		}

		user, err := s.store.FindUser(r.Context(), raw)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				s.writeGraphQLAuthError(w, headerUserID+" does not name a known user")
				return
			}
			s.fail(w, r, "look up graphql caller", err)
			return
		}

		ctx := graph.WithUser(r.Context(), user)
		ctx = graph.WithLoaders(ctx, graph.NewLoaders(s.store))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// writeGraphQLAuthError answers with a GraphQL error envelope. The status stays 200 because the
// failure is in the response body, where a GraphQL client looks for it.
func (s *server) writeGraphQLAuthError(w http.ResponseWriter, message string) {
	writeJSON(w, s.log, http.StatusOK, map[string]any{
		"data": nil,
		"errors": []map[string]any{{
			"message":    message,
			"extensions": map[string]any{"code": graph.CodeUnauthenticated},
		}},
	})
}

package directives

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/nais/api/internal/auth/authz"
)

// Auth Make sure there is an authenticated user making this request.
func Auth() DirectiveFunc {
	return func(ctx context.Context, obj interface{}, next graphql.Resolver) (interface{}, error) {
		actor := authz.ActorFromContext(ctx)
		if !actor.Authenticated() {
			return nil, authz.ErrNotAuthenticated
		}

		return next(ctx)
	}
}

package directives

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database/gensql"
)

// Admin Require a user with the admin role to allow the request
func Admin() DirectiveFunc {
	return func(ctx context.Context, obj interface{}, next graphql.Resolver) (interface{}, error) {
		actor := authz.ActorFromContext(ctx)
		if !actor.Authenticated() {
			return nil, authz.ErrNotAuthenticated
		}

		err := authz.RequireRole(actor, gensql.RoleNameAdmin)
		if err != nil {
			return nil, err
		}

		return next(ctx)
	}
}

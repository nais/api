package directives

import (
	"context"

	"github.com/nais/api/internal/sqlc"

	"github.com/99designs/gqlgen/graphql"
	"github.com/nais/api/internal/authz"
)

// Admin Require a user with the admin role to allow the request
func Admin() DirectiveFunc {
	return func(ctx context.Context, obj interface{}, next graphql.Resolver) (interface{}, error) {
		actor := authz.ActorFromContext(ctx)
		if !actor.Authenticated() {
			return nil, authz.ErrNotAuthenticated
		}

		err := authz.RequireRole(actor, sqlc.RoleNameAdmin)
		if err != nil {
			return nil, err
		}

		return next(ctx)
	}
}

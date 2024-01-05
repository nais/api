package directives

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
)

type DirectiveFunc func(ctx context.Context, obj interface{}, next graphql.Resolver) (res interface{}, err error)

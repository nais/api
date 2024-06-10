package graph

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
)

func (r *queryResolver) Me(ctx context.Context) (model.AuthenticatedUser, error) {
	me := authz.ActorFromContext(ctx).User

	switch me := me.(type) {
	case *database.User:
		return &model.User{
			ID:         scalar.UserIdent(me.ID),
			Email:      me.Email,
			Name:       me.Name,
			ExternalID: me.ExternalID,
		}, nil
	case *database.ServiceAccount:
		return &model.ServiceAccount{
			ID:   me.ID,
			Name: me.Name,
		}, nil
	default:
		return nil, apierror.Errorf("unknown user type: %T", me)
	}
}

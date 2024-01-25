package graph

import (
	"cmp"
	"context"
	"github.com/nais/api/internal/graph/apierror"
	"slices"

	"github.com/nais/api/internal/auth/authz"
	sqlc "github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
)

func convertSecretDataToTuple(data map[string]string) []*model.SecretTuple {
	ret := make([]*model.SecretTuple, 0, len(data))
	for key, value := range data {
		ret = append(ret, &model.SecretTuple{
			Key:   key,
			Value: value,
		})
	}
	slices.SortFunc(ret, func(a, b *model.SecretTuple) int {
		return cmp.Compare(a.Key, b.Key)
	})
	return ret
}

func requireTeamMemberOrOwner(ctx context.Context, team slug.Slug) error {
	actor := authz.ActorFromContext(ctx)
	isMember := false
	isOwner := false

	err := authz.RequireTeamRole(actor, team, sqlc.RoleNameTeammember)
	if err == nil {
		isMember = true
	}

	err = authz.RequireTeamRole(actor, team, sqlc.RoleNameTeamowner)
	if err == nil {
		isOwner = true
	}

	if isMember || isOwner {
		return nil
	}

	return apierror.Errorf("User is not a member or owner of %q", team)
}

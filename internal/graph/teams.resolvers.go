package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/roles"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
	"github.com/sourcegraph/conc/pool"
	"k8s.io/utils/ptr"
)

func (r *queryResolver) Teams(ctx context.Context, offset *int, limit *int) (*model.TeamList, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireGlobalAuthorization(actor, roles.AuthorizationTeamsList)
	if err != nil {
		return nil, err
	}

	var teams []*database.Team

	p := model.NewPagination(offset, limit)
	var pageInfo model.PageInfo

	var total int
	teams, total, err = r.database.GetTeams(ctx, database.Page{
		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return nil, err
	}

	pageInfo = model.NewPageInfo(p, total)

	return &model.TeamList{
		Nodes:    toGraphTeams(teams),
		PageInfo: pageInfo,
	}, nil
}

func (r *queryResolver) Team(ctx context.Context, slug slug.Slug) (*model.Team, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamAuthorization(actor, roles.AuthorizationTeamsRead, slug)
	if err != nil {
		return nil, err
	}

	return loader.GetTeam(ctx, slug)
}

func (r *teamResolver) ID(ctx context.Context, obj *model.Team) (*scalar.Ident, error) {
	return ptr.To(scalar.TeamIdent(obj.Slug)), nil
}

func (r *teamResolver) DeletionInProgress(ctx context.Context, obj *model.Team) (bool, error) {
	return obj.DeleteKeyConfirmedAt != nil, nil
}

func (r *teamResolver) ResourceInventory(ctx context.Context, obj *model.Team) (*model.ResourceInventory, error) {
	wg := pool.NewWithResults[any]().WithErrors().WithFirstError()
	results := make(map[string]int)
	wg.Go(func() (any, error) {
		apps, err := r.k8sClient.Apps(ctx, obj.Slug.String())
		if err != nil {
			return nil, fmt.Errorf("getting apps from Kubernetes: %w", err)
		}
		results["apps"] = len(apps)
		return results, nil
	})

	wg.Go(func() (any, error) {
		jobs, err := r.k8sClient.NaisJobs(ctx, obj.Slug.String())
		if err != nil {
			return nil, fmt.Errorf("getting naisjobs from Kubernetes: %w", err)
		}
		results["jobs"] = len(jobs)
		return results, nil
	})

	wgRes, err := wg.Wait()
	if err != nil {
		return nil, err
	}

	inventory := &model.ResourceInventory{}
	inventory.IsEmpty = true
	for _, result := range wgRes {
		for k, v := range result.(map[string]int) {
			switch k {
			case "apps":
				inventory.TotalApps = v
			case "jobs":
				inventory.TotalJobs = v
			}
			if v > 0 {
				inventory.IsEmpty = false
			}
		}
	}

	return inventory, nil
}

func (r *teamResolver) Environments(ctx context.Context, obj *model.Team) ([]*model.Env, error) {
	// Env is a bit special, given that it will be created from k8s etc.
	// All fields, except name and team, are resolved.

	dbEnvs, _, err := r.database.GetTeamEnvironments(ctx, obj.Slug, database.Page{Limit: 50})
	if err != nil {
		return nil, err
	}

	ret := make([]*model.Env, len(dbEnvs))
	for i, env := range dbEnvs {
		ret[i] = &model.Env{Name: env.Environment, Team: obj.Slug.String()}
	}

	return ret, nil
}

func (r *Resolver) Team() gengql.TeamResolver { return &teamResolver{r} }

type teamResolver struct{ *Resolver }

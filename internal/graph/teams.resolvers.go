package graph

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/roles"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/thirdparty/hookd"
	"github.com/nais/api/pkg/apiclient/protoapi"
	"github.com/sourcegraph/conc/pool"
	"k8s.io/utils/ptr"
)

func (r *appUtilizationDataResolver) App(ctx context.Context, obj *model.AppUtilizationData) (*model.App, error) {
	app, err := r.k8sClient.App(ctx, obj.AppName, obj.TeamSlug.String(), obj.Env)
	if err != nil {
		r.log.Errorf("getting app %s in team %s: %v", obj.AppName, obj.TeamSlug, err)
		return nil, apierror.ErrAppNotFound
	}

	return app, nil
}

func (r *mutationResolver) UpdateTeamSlackAlertsChannel(ctx context.Context, slug slug.Slug, input model.UpdateTeamSlackAlertsChannelInput) (*model.Team, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamAuthorization(actor, roles.AuthorizationTeamsMetadataUpdate, slug)
	if err != nil {
		return nil, err
	}

	input = input.Sanitize()
	err = input.Validate(r.clusters.Names())
	if err != nil {
		return nil, err
	}

	correlationID := uuid.New()
	err = r.database.UpsertTeamEnvironment(ctx, slug, input.Environment, input.ChannelName, nil)
	if err != nil {
		return nil, err
	}

	r.triggerTeamUpdatedEvent(ctx, slug, correlationID)

	return loader.GetTeam(ctx, slug)
}

func (r *mutationResolver) SynchronizeTeam(ctx context.Context, slug slug.Slug) (*model.TeamSync, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamAuthorization(actor, roles.AuthorizationTeamsSynchronize, slug)
	if err != nil {
		return nil, err
	}

	if _, err := loader.GetTeam(ctx, slug); err != nil {
		return nil, err
	}

	correlationID := uuid.New()

	r.triggerTeamUpdatedEvent(ctx, slug, correlationID)

	return &model.TeamSync{
		CorrelationID: correlationID,
	}, nil
}

func (r *mutationResolver) SynchronizeAllTeams(ctx context.Context) (*model.TeamSync, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireGlobalAuthorization(actor, roles.AuthorizationTeamsSynchronize)
	if err != nil {
		return nil, err
	}

	correlationID := uuid.New()
	r.triggerEvent(ctx, protoapi.EventTypes_EVENT_SYNC_ALL_TEAMS, &protoapi.EventSyncAllTeams{}, correlationID)

	return &model.TeamSync{
		CorrelationID: correlationID,
	}, nil
}

func (r *mutationResolver) ChangeDeployKey(ctx context.Context, team slug.Slug) (*model.DeploymentKey, error) {
	actor := authz.ActorFromContext(ctx)
	if _, err := r.database.GetTeamMember(ctx, team, actor.User.GetID()); errors.Is(err, pgx.ErrNoRows) {
		return nil, apierror.ErrUserIsNotTeamMember
	} else if err != nil {
		return nil, err
	}

	deployKey, err := r.hookdClient.ChangeDeployKey(ctx, team.String())
	if err != nil {
		return nil, fmt.Errorf("changing deploy key in Hookd: %w", err)
	}

	return &model.DeploymentKey{
		ID:      scalar.DeployKeyIdent(team),
		Key:     deployKey.Key,
		Created: deployKey.Created,
		Expires: deployKey.Expires,
	}, nil
}

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

func (r *queryResolver) TeamsUtilization(ctx context.Context, resourceType model.UsageResourceType) ([]*model.TeamUtilizationData, error) {
	return r.resourceUsageClient.TeamsUtilization(ctx, resourceType)
}

func (r *teamResolver) ID(ctx context.Context, obj *model.Team) (*scalar.Ident, error) {
	return ptr.To(scalar.TeamIdent(obj.Slug)), nil
}

func (r *teamResolver) Members(ctx context.Context, obj *model.Team, offset *int, limit *int) (*model.TeamMemberList, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireGlobalAuthorization(actor, roles.AuthorizationUsersList)
	if err != nil {
		return nil, err
	}

	p := model.NewPagination(offset, limit)

	users, total, err := r.database.GetTeamMembers(ctx, obj.Slug, database.Page{
		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return nil, err
	}

	members := make([]*model.TeamMember, len(users))
	for idx, user := range users {
		members[idx] = &model.TeamMember{
			UserID:   scalar.UserIdent(user.ID),
			TeamSlug: obj.Slug,
		}
	}

	return &model.TeamMemberList{
		Nodes:    members,
		PageInfo: model.NewPageInfo(p, total),
	}, nil
}

func (r *teamResolver) Member(ctx context.Context, obj *model.Team, userID scalar.Ident) (*model.TeamMember, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireGlobalAuthorization(actor, roles.AuthorizationUsersList)
	if err != nil {
		return nil, err
	}

	uid, err := userID.AsUUID()
	if err != nil {
		return nil, err
	}

	user, err := r.database.GetUserByID(ctx, uid)
	if err != nil {
		return nil, apierror.ErrUserNotExists
	}

	return &model.TeamMember{
		UserID:   scalar.UserIdent(user.ID),
		TeamSlug: obj.Slug,
	}, nil
}

func (r *teamResolver) DeletionInProgress(ctx context.Context, obj *model.Team) (bool, error) {
	return obj.DeleteKeyConfirmedAt != nil, nil
}

func (r *teamResolver) ViewerIsOwner(ctx context.Context, obj *model.Team) (bool, error) {
	actor := authz.ActorFromContext(ctx)
	return r.database.UserIsTeamOwner(ctx, actor.User.GetID(), obj.Slug)
}

func (r *teamResolver) ViewerIsMember(ctx context.Context, obj *model.Team) (bool, error) {
	actor := authz.ActorFromContext(ctx)
	u, err := r.database.GetTeamMember(ctx, obj.Slug, actor.User.GetID())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return u != nil, nil
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

func (r *teamResolver) Apps(ctx context.Context, obj *model.Team, offset *int, limit *int, orderBy *model.OrderBy) (*model.AppList, error) {
	apps, err := r.k8sClient.Apps(ctx, obj.Slug.String())
	if err != nil {
		return nil, fmt.Errorf("getting apps from Kubernetes: %w", err)
	}

	if orderBy != nil {
		switch orderBy.Field {
		case model.OrderByFieldName:
			model.SortWith(apps, func(a, b *model.App) bool {
				return model.Compare(a.Name, b.Name, orderBy.Direction)
			})
		case model.OrderByFieldEnv:
			model.SortWith(apps, func(a, b *model.App) bool {
				return model.Compare(a.Env.Name, b.Env.Name, orderBy.Direction)
			})
		case model.OrderByFieldDeployed:
			model.SortWith(apps, func(a, b *model.App) bool {
				if a.DeployInfo.Timestamp == nil {
					return false
				}
				if b.DeployInfo.Timestamp == nil {
					return true
				}
				return model.Compare(b.DeployInfo.Timestamp.UnixMilli(), a.DeployInfo.Timestamp.UnixMilli(), orderBy.Direction)
			})
		case model.OrderByFieldStatus:
			model.SortWith(apps, func(a, b *model.App) bool {
				sortOrder := []model.State{model.StateFailing, model.StateNotnais, model.StateUnknown, model.StateNais}
				aIndex := -1
				bIndex := -1
				for i, s := range sortOrder {
					if a.Status.State == s {
						aIndex = i
					}
					if b.Status.State == s {
						bIndex = i
					}
				}
				if aIndex == -1 {
					return false
				}
				if bIndex == -1 {
					return true
				}
				if orderBy.Direction == model.SortOrderAsc {
					return aIndex < bIndex
				}
				return aIndex > bIndex
			})
		}
	}

	pagination := model.NewPagination(offset, limit)
	apps, pageInfo := model.PaginatedSlice(apps, pagination)
	for _, app := range apps {
		app.GQLVars = model.WorkloadBaseGQLVars{Team: obj.Slug}
	}

	return &model.AppList{
		Nodes:    apps,
		PageInfo: pageInfo,
	}, nil
}

func (r *teamResolver) DeployKey(ctx context.Context, obj *model.Team) (*model.DeploymentKey, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamAuthorization(actor, roles.AuthorizationDeployKeyView, obj.Slug)
	if err != nil {
		if actor.User.IsServiceAccount() {
			return nil, apierror.ErrUserIsNotTeamMember
		}
		return nil, err
	}

	key, err := r.hookdClient.DeployKey(ctx, obj.Slug.String())
	if err != nil {
		return nil, fmt.Errorf("getting deploy key from Hookd: %w", err)
	}

	return &model.DeploymentKey{
		ID:      scalar.DeployKeyIdent(obj.Slug),
		Key:     key.Key,
		Created: key.Created,
		Expires: key.Expires,
	}, nil
}

func (r *teamResolver) Naisjobs(ctx context.Context, obj *model.Team, offset *int, limit *int, orderBy *model.OrderBy) (*model.NaisJobList, error) {
	naisjobs, err := r.k8sClient.NaisJobs(ctx, obj.Slug.String())
	if err != nil {
		return nil, fmt.Errorf("getting naisjobs from Kubernetes: %w", err)
	}

	if orderBy != nil {
		switch orderBy.Field {
		case model.OrderByFieldName:
			model.SortWith(naisjobs, func(a, b *model.NaisJob) bool {
				return model.Compare(a.Name, b.Name, orderBy.Direction)
			})
		case model.OrderByFieldEnv:
			model.SortWith(naisjobs, func(a, b *model.NaisJob) bool {
				return model.Compare(a.Env.Name, b.Env.Name, orderBy.Direction)
			})
		case model.OrderByFieldDeployed:
			model.SortWith(naisjobs, func(a, b *model.NaisJob) bool {
				if a.DeployInfo.Timestamp == nil {
					return false
				}
				if b.DeployInfo.Timestamp == nil {
					return true
				}
				return model.Compare(b.DeployInfo.Timestamp.UnixMilli(), a.DeployInfo.Timestamp.UnixMilli(), orderBy.Direction)
			})
		case model.OrderByFieldStatus:
			model.SortWith(naisjobs, func(a, b *model.NaisJob) bool {
				sortOrder := []model.State{model.StateFailing, model.StateNotnais, model.StateUnknown, model.StateNais}
				aIndex := -1
				bIndex := -1
				for i, s := range sortOrder {
					if a.Status.State == s {
						aIndex = i
					}
					if b.Status.State == s {
						bIndex = i
					}
				}
				if aIndex == -1 {
					return false
				}
				if bIndex == -1 {
					return true
				}
				if orderBy.Direction == model.SortOrderAsc {
					return aIndex < bIndex
				}
				return aIndex > bIndex
			})
		}
	}

	pagination := model.NewPagination(offset, limit)
	jobs, pageInfo := model.PaginatedSlice(naisjobs, pagination)
	for _, job := range jobs {
		job.GQLVars = model.WorkloadBaseGQLVars{Team: obj.Slug}
	}

	return &model.NaisJobList{
		Nodes:    jobs,
		PageInfo: pageInfo,
	}, nil
}

func (r *teamResolver) Deployments(ctx context.Context, obj *model.Team, offset *int, limit *int) (*model.DeploymentList, error) {
	pagination := model.NewPagination(offset, limit)

	deploys, err := r.hookdClient.Deployments(ctx, hookd.WithTeam(obj.Slug.String()), hookd.WithLimit(pagination.Limit))
	if err != nil {
		return nil, fmt.Errorf("getting deploys from Hookd: %w", err)
	}

	return &model.DeploymentList{
		Nodes: deployToModel(deploys),
		PageInfo: model.PageInfo{
			HasNextPage:     len(deploys) >= pagination.Limit,
			HasPreviousPage: pagination.Offset > 0,
			TotalCount:      0,
		},
	}, nil
}

func (r *teamResolver) Secrets(ctx context.Context, obj *model.Team) ([]*model.Secret, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamMembership(actor, obj.Slug)
	if err != nil {
		return nil, err
	}
	return r.k8sClient.Secrets(ctx, obj.Slug)
}

func (r *teamResolver) Secret(ctx context.Context, obj *model.Team, name string, env string) (*model.Secret, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamMembership(actor, obj.Slug)
	if err != nil {
		return nil, err
	}
	return r.k8sClient.Secret(ctx, name, obj.Slug, env)
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

func (r *teamResolver) Unleash(ctx context.Context, obj *model.Team) (*model.Unleash, error) {
	return r.unleashMgr.Unleash(obj.Slug.String())
}

func (r *teamResolver) AppsUtilization(ctx context.Context, obj *model.Team, resourceType model.UsageResourceType) ([]*model.AppUtilizationData, error) {
	return r.resourceUsageClient.TeamUtilization(ctx, obj.Slug, resourceType)
}

func (r *teamMemberResolver) Team(ctx context.Context, obj *model.TeamMember) (*model.Team, error) {
	return loader.GetTeam(ctx, obj.TeamSlug)
}

func (r *teamMemberResolver) User(ctx context.Context, obj *model.TeamMember) (*model.User, error) {
	uid, err := obj.UserID.AsUUID()
	if err != nil {
		return nil, err
	}
	return loader.GetUser(ctx, uid)
}

func (r *teamMemberResolver) Role(ctx context.Context, obj *model.TeamMember) (model.TeamRole, error) {
	if obj.TeamRole != "" {
		return obj.TeamRole, nil
	}
	uid, err := obj.UserID.AsUUID()
	if err != nil {
		return "", err
	}

	isOwner, err := r.database.UserIsTeamOwner(ctx, uid, obj.TeamSlug)
	if err != nil {
		return "", err
	}

	role := model.TeamRoleMember
	if isOwner {
		role = model.TeamRoleOwner
	}

	return role, nil
}

func (r *teamUtilizationDataResolver) Team(ctx context.Context, obj *model.TeamUtilizationData) (*model.Team, error) {
	r.log.Infof("first teamUtilizationDataResolver.Team: %v", obj.TeamSlug)

	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamAuthorization(actor, roles.AuthorizationTeamsRead, obj.TeamSlug)
	if err != nil {
		return nil, err
	}

	team, err := loader.GetTeam(ctx, obj.TeamSlug)
	if err != nil {
		r.log.WithError(err).Error("get team error teamUtilizationDataResolver.Team ", "teamSlug", obj.TeamSlug)
	}

	if team == nil {
		r.log.Info("team is nil - teamUtilizationDataResolver.Team ", "teamSlug", obj.TeamSlug, "team", team)
	}

	return team, err
}

func (r *Resolver) AppUtilizationData() gengql.AppUtilizationDataResolver {
	return &appUtilizationDataResolver{r}
}

func (r *Resolver) Team() gengql.TeamResolver { return &teamResolver{r} }

func (r *Resolver) TeamMember() gengql.TeamMemberResolver { return &teamMemberResolver{r} }

func (r *Resolver) TeamUtilizationData() gengql.TeamUtilizationDataResolver {
	return &teamUtilizationDataResolver{r}
}

type (
	appUtilizationDataResolver  struct{ *Resolver }
	teamResolver                struct{ *Resolver }
	teamMemberResolver          struct{ *Resolver }
	teamUtilizationDataResolver struct{ *Resolver }
)

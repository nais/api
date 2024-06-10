package graph

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auditlogger"
	"github.com/nais/api/internal/auditlogger/audittype"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/roles"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"k8s.io/utils/ptr"
)

func (r *mutationResolver) SynchronizeUsers(ctx context.Context) (string, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireGlobalAuthorization(actor, roles.AuthorizationUsersyncSynchronize)
	if err != nil {
		return "", err
	}

	correlationID := uuid.New()

	targets := []auditlogger.Target{
		auditlogger.SystemTarget("usersync"),
	}
	fields := auditlogger.Fields{
		Action:        audittype.AuditActionGraphqlApiUsersSync,
		Actor:         actor,
		CorrelationID: correlationID,
	}
	r.auditLogger.Logf(ctx, targets, fields, "Trigger user sync")
	r.usersyncTrigger <- correlationID

	return correlationID.String(), nil
}

func (r *queryResolver) Users(ctx context.Context, offset *int, limit *int) (*model.UserList, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireGlobalAuthorization(actor, roles.AuthorizationUsersList)
	if err != nil {
		return nil, err
	}
	p := model.NewPagination(offset, limit)
	users, total, err := r.database.GetUsers(ctx, database.Page{
		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return nil, err
	}

	ret := make([]*model.User, 0, len(users))
	for _, u := range users {
		ret = append(ret, loader.ToGraphUser(u))
	}

	return &model.UserList{
			Nodes:    ret,
			PageInfo: model.NewPageInfo(p, total),
		},
		nil
}

func (r *queryResolver) User(ctx context.Context, id *scalar.Ident, email *string) (*model.User, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireGlobalAuthorization(actor, roles.AuthorizationUsersList)
	if err != nil {
		return nil, err
	}

	if id != nil {
		uid, err := id.AsUUID()
		if err != nil {
			return nil, err
		}
		return loader.GetUser(ctx, uid)
	}
	if email != nil {
		u, err := r.database.GetUserByEmail(ctx, *email)
		if err != nil {
			return nil, err
		}
		return loader.ToGraphUser(u), nil
	}
	return nil, apierror.Errorf("Either id or email must be specified")
}

func (r *queryResolver) UsersyncRuns(ctx context.Context, limit *int, offset *int) (*model.UsersyncRunList, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireGlobalAuthorization(actor, roles.AuthorizationUsersyncSynchronize)
	if err != nil {
		return nil, err
	}
	p := model.NewPagination(offset, limit)
	runs, total, err := r.database.GetUsersyncRuns(ctx, database.Page{
		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return nil, err
	}

	ret := make([]*model.UsersyncRun, len(runs))
	for i, run := range runs {
		ret[i] = &model.UsersyncRun{
			ID:         scalar.UsersyncRunIdent(run.ID),
			StartedAt:  run.StartedAt.Time,
			FinishedAt: run.FinishedAt.Time,
			Error:      run.Error,
			GQLVars: model.UsersyncRunGQLVars{
				CorrelationID: run.ID,
			},
		}
	}

	return &model.UsersyncRunList{
		Nodes:    ret,
		PageInfo: model.NewPageInfo(p, total),
	}, nil
}

func (r *userResolver) Teams(ctx context.Context, obj *model.User, limit *int, offset *int) (*model.TeamMemberList, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireGlobalAuthorization(actor, roles.AuthorizationTeamsList)
	if err != nil {
		return nil, err
	}
	p := model.NewPagination(offset, limit)
	uid, err := obj.ID.AsUUID()
	if err != nil {
		return nil, err
	}

	userTeams, totalCount, err := r.database.GetUserTeamsPaginated(ctx, uid, database.Page{
		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return nil, err
	}

	teams := make([]*model.TeamMember, 0)
	for _, userTeam := range userTeams {
		var teamRole model.TeamRole
		switch userTeam.RoleName {
		case gensql.RoleNameTeammember:
			teamRole = model.TeamRoleMember
		case gensql.RoleNameTeamowner:
			teamRole = model.TeamRoleOwner
		default:
			continue
		}
		teams = append(teams, &model.TeamMember{
			TeamRole: teamRole,
			TeamSlug: userTeam.Team.Slug,
			UserID:   obj.ID,
		})
	}

	return &model.TeamMemberList{
		PageInfo: model.NewPageInfo(p, totalCount),
		Nodes:    teams,
	}, nil
}

func (r *userResolver) Roles(ctx context.Context, obj *model.User) ([]*model.Role, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireRole(actor, gensql.RoleNameAdmin)
	if err != nil {
		return nil, err
	}

	uid, err := obj.ID.AsUUID()
	if err != nil {
		return nil, err
	}

	if actor.User.GetID() != uid {
		return nil, err
	}

	return loader.GetUserRoles(ctx, uid)
}

func (r *userResolver) IsAdmin(ctx context.Context, obj *model.User) (*bool, error) {
	uid, err := obj.ID.AsUUID()
	if err != nil {
		return nil, err
	}

	userRoles, err := loader.GetUserRoles(ctx, uid)
	if err != nil {
		return nil, err
	}

	for _, ur := range userRoles {
		if ur.Name == string(gensql.RoleNameAdmin) {
			return ptr.To(true), nil
		}
	}

	return ptr.To(false), nil
}

func (r *usersyncRunResolver) AuditLogs(ctx context.Context, obj *model.UsersyncRun, limit *int, offset *int) (*model.AuditLogList, error) {
	p := model.NewPagination(offset, limit)
	entries, total, err := r.database.GetAuditLogsForCorrelationID(ctx, obj.GQLVars.CorrelationID, database.Page{
		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return nil, err
	}

	return &model.AuditLogList{
		Nodes:    toGraphAuditLogs(entries),
		PageInfo: model.NewPageInfo(p, total),
	}, nil
}

func (r *usersyncRunResolver) Status(ctx context.Context, obj *model.UsersyncRun) (model.UsersyncRunStatus, error) {
	if obj.Error == nil {
		return model.UsersyncRunStatusSuccess, nil
	}

	return model.UsersyncRunStatusFailure, nil
}

func (r *Resolver) User() gengql.UserResolver { return &userResolver{r} }

func (r *Resolver) UsersyncRun() gengql.UsersyncRunResolver { return &usersyncRunResolver{r} }

type (
	userResolver        struct{ *Resolver }
	usersyncRunResolver struct{ *Resolver }
)

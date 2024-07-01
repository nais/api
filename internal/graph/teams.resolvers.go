package graph

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
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
	"github.com/nais/api/internal/graph/model/auditevent"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/thirdparty/hookd"
	"github.com/nais/api/pkg/protoapi"
	"github.com/sourcegraph/conc/pool"
	"k8s.io/utils/ptr"
)

func (r *mutationResolver) CreateTeam(ctx context.Context, input model.CreateTeamInput) (*model.Team, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireGlobalAuthorization(actor, roles.AuthorizationTeamsCreate)
	if err != nil {
		return nil, err
	}

	input = input.Sanitize()

	err = input.Validate()
	if err != nil {
		return nil, err
	}

	correlationID := uuid.New()

	var team *database.Team
	err = r.database.Transaction(ctx, func(ctx context.Context, dbtx database.Database) error {
		team, err = dbtx.CreateTeam(ctx, input.Slug, input.Purpose, input.SlackChannel)
		if err != nil {
			return err
		}

		if actor.User.IsServiceAccount() {
			return dbtx.AssignTeamRoleToServiceAccount(ctx, actor.User.GetID(), gensql.RoleNameTeamowner, input.Slug)
		}

		return dbtx.SetTeamMemberRole(ctx, actor.User.GetID(), team.Slug, gensql.RoleNameTeamowner)
	})
	if err != nil {
		return nil, err
	}

	err = r.auditor.TeamCreated(ctx, actor.User, team.Slug)
	if err != nil {
		return nil, err
	}

	r.triggerTeamUpdatedEvent(ctx, team.Slug, correlationID)

	return loader.ToGraphTeam(team), nil
}

func (r *mutationResolver) UpdateTeam(ctx context.Context, slug slug.Slug, input model.UpdateTeamInput) (*model.Team, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamAuthorization(actor, roles.AuthorizationTeamsMetadataUpdate, slug)
	if err != nil {
		return nil, err
	}

	if _, err := loader.GetTeam(ctx, slug); err != nil {
		return nil, err
	}

	input = input.Sanitize()
	err = input.Validate()
	if err != nil {
		return nil, err
	}

	correlationID := uuid.New()
	var team *database.Team
	team, err = r.database.UpdateTeam(ctx, slug, input.Purpose, input.SlackChannel)
	if err != nil {
		return nil, err
	}

	if input.Purpose != nil {
		err = r.auditor.TeamSetPurpose(ctx, actor.User, slug, *input.Purpose)
		if err != nil {
			return nil, err
		}
	}

	if input.SlackChannel != nil {
		err = r.auditor.TeamSetDefaultSlackChannel(ctx, actor.User, slug, *input.SlackChannel)
		if err != nil {
			return nil, err
		}
	}

	r.triggerTeamUpdatedEvent(ctx, team.Slug, correlationID)

	return loader.ToGraphTeam(team), nil
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

	err = r.auditor.TeamSetAlertsSlackChannel(ctx, actor.User, slug, input.Environment, *input.ChannelName)
	if err != nil {
		return nil, err
	}

	r.triggerTeamUpdatedEvent(ctx, slug, correlationID)

	return loader.GetTeam(ctx, slug)
}

func (r *mutationResolver) RemoveUserFromTeam(ctx context.Context, slug slug.Slug, userID scalar.Ident) (*model.Team, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamAuthorization(actor, roles.AuthorizationTeamsMembersAdmin, slug)
	if err != nil {
		return nil, err
	}

	userUID, err := userID.AsUUID()
	if err != nil {
		return nil, err
	}

	team, err := loader.GetTeam(ctx, slug)
	if err != nil {
		return nil, err
	}

	correlationID := uuid.New()

	var member *database.User
	err = r.database.Transaction(ctx, func(ctx context.Context, dbtx database.Database) error {
		members, err := dbtx.GetAllTeamMembers(ctx, slug)
		if err != nil {
			return fmt.Errorf("get team members of %q: %w", slug, err)
		}

		memberFromUserID := func(userId uuid.UUID) *database.User {
			for _, m := range members {
				if m.ID == userId {
					return m
				}
			}
			return nil
		}

		member = memberFromUserID(userUID)
		if member == nil {
			return apierror.Errorf("The user %q is not a member of team %q.", userUID, slug)
		}

		err = dbtx.RemoveUserFromTeam(ctx, userUID, slug)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	err = r.auditor.TeamMemberRemoved(ctx, actor.User, slug, member.Email)
	if err != nil {
		return nil, err
	}

	r.triggerTeamUpdatedEvent(ctx, slug, correlationID)

	return team, nil
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

	if err := r.auditor.TeamSynchronized(ctx, actor.User, slug); err != nil {
		return nil, err
	}

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

	limit, offset := 100, 0
	teams := make([]*database.Team, 0)
	for {
		page, _, err := r.database.GetTeams(ctx, database.Page{
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			return nil, err
		}
		teams = append(teams, page...)
		if len(page) < limit {
			break
		}
		offset += limit
	}

	targets := make([]auditlogger.Target, 0, len(teams))
	for _, entry := range teams {
		targets = append(targets, auditlogger.TeamTarget(entry.Slug))
	}
	fields := auditlogger.Fields{
		Action:        audittype.AuditActionGraphqlApiTeamSync,
		Actor:         actor,
		CorrelationID: correlationID,
	}
	r.auditLogger.Logf(ctx, targets, fields, "Manually scheduled for synchronization")
	r.triggerEvent(ctx, protoapi.EventTypes_EVENT_SYNC_ALL_TEAMS, &protoapi.EventSyncAllTeams{}, correlationID)

	return &model.TeamSync{
		CorrelationID: correlationID,
	}, nil
}

func (r *mutationResolver) AddTeamMember(ctx context.Context, slug slug.Slug, member model.TeamMemberInput) (*model.Team, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamAuthorization(actor, roles.AuthorizationTeamsMembersAdmin, slug)
	if err != nil {
		return nil, err
	}
	team, err := loader.GetTeam(ctx, slug)
	if err != nil {
		return nil, err
	}

	uid, err := member.UserID.AsUUID()
	if err != nil {
		return nil, err
	}

	user, err := r.database.GetUserByID(ctx, uid)
	if err != nil {
		return nil, apierror.ErrUserNotExists
	}

	correlationID := uuid.New()

	err = r.database.Transaction(ctx, func(ctx context.Context, dbtx database.Database) error {
		teamMember, _ := dbtx.GetTeamMember(ctx, slug, user.ID)
		if teamMember != nil {
			return apierror.Errorf("User is already a member of the team.")
		}

		role, err := gensqlRoleFromTeamRole(member.Role)
		if err != nil {
			return err
		}

		err = dbtx.SetTeamMemberRole(ctx, user.ID, team.Slug, role)
		if err != nil {
			return err
		}

		for _, reconcilerName := range member.ReconcilerOptOuts {
			err = dbtx.AddReconcilerOptOut(ctx, user.ID, team.Slug, reconcilerName)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	err = r.auditor.TeamMemberAdded(ctx, actor.User, slug, user.Email, member.Role)
	if err != nil {
		return nil, err
	}

	r.triggerTeamUpdatedEvent(ctx, team.Slug, correlationID)

	return team, nil
}

func (r *mutationResolver) SetTeamMemberRole(ctx context.Context, slug slug.Slug, userID scalar.Ident, role model.TeamRole) (*model.Team, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamAuthorization(actor, roles.AuthorizationTeamsMembersAdmin, slug)
	if err != nil {
		return nil, err
	}

	uid, err := userID.AsUUID()
	if err != nil {
		return nil, err
	}

	team, err := loader.GetTeam(ctx, slug)
	if err != nil {
		return nil, err
	}

	correlationID := uuid.New()

	members, err := r.database.GetAllTeamMembers(ctx, team.Slug)
	if err != nil {
		return nil, fmt.Errorf("get team members: %w", err)
	}

	var member *database.User = nil
	for _, m := range members {
		if m.ID == uid {
			member = m
			break
		}
	}
	if member == nil {
		return nil, fmt.Errorf("user %q not in team %q", uid, slug)
	}

	desiredRole, err := gensqlRoleFromTeamRole(role)
	if err != nil {
		return nil, err
	}

	err = r.database.Transaction(ctx, func(ctx context.Context, dbtx database.Database) error {
		err = dbtx.RemoveUserFromTeam(ctx, uid, team.Slug)
		if err != nil {
			return err
		}

		err = dbtx.SetTeamMemberRole(ctx, uid, team.Slug, desiredRole)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := r.auditor.TeamMemberSetRole(ctx, actor.User, slug, member.Email, role); err != nil {
		return nil, err
	}

	r.triggerTeamUpdatedEvent(ctx, team.Slug, correlationID)

	return team, nil
}

func (r *mutationResolver) RequestTeamDeletion(ctx context.Context, slug slug.Slug) (*model.TeamDeleteKey, error) {
	actor := authz.ActorFromContext(ctx)

	err := authz.RequireTeamRole(actor, slug, gensql.RoleNameTeamowner)
	if err != nil {
		return nil, err
	}

	team, err := loader.GetTeam(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.ErrTeamNotExist
		}
		return nil, err
	}

	deleteKey, err := r.database.CreateTeamDeleteKey(ctx, slug, actor.User.GetID())
	if err != nil {
		return nil, fmt.Errorf("create team delete key: %w", err)
	}

	err = r.auditor.TeamDeletionRequested(ctx, actor.User, team.Slug)
	if err != nil {
		return nil, err
	}

	return toGraphTeamDeleteKey(deleteKey), nil
}

func (r *mutationResolver) ConfirmTeamDeletion(ctx context.Context, key string) (bool, error) {
	uid, err := uuid.Parse(key)
	if err != nil {
		return false, apierror.Errorf("Invalid deletion key: %q", key)
	}

	deleteKey, err := r.database.GetTeamDeleteKey(ctx, uid)
	if err != nil {
		return false, apierror.Errorf("Unknown deletion key: %q", key)
	}

	actor := authz.ActorFromContext(ctx)
	err = authz.RequireTeamRole(actor, deleteKey.TeamSlug, gensql.RoleNameTeamowner)
	if err != nil {
		return false, err
	}

	if actor.User.GetID() == deleteKey.CreatedBy {
		return false, apierror.Errorf("You cannot confirm your own delete key.")
	}

	if deleteKey.ConfirmedAt.Valid {
		return false, apierror.Errorf("Key has already been confirmed, team is currently being deleted.")
	}

	if deleteKey.HasExpired() {
		return false, apierror.Errorf("Team delete key has expired, you need to request a new key.")
	}

	correlationID := uuid.New()

	err = r.database.ConfirmTeamDeleteKey(ctx, uid)
	if err != nil {
		return false, fmt.Errorf("confirm team delete key: %w", err)
	}

	err = r.auditor.TeamDeletionConfirmed(ctx, actor.User, deleteKey.TeamSlug)
	if err != nil {
		return false, err
	}

	r.triggerTeamDeletedEvent(ctx, deleteKey.TeamSlug, correlationID)

	return true, nil
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

	err = r.auditor.TeamRotatedDeployKey(ctx, actor.User, team)
	if err != nil {
		return nil, err
	}

	return &model.DeploymentKey{
		ID:      scalar.DeployKeyIdent(team),
		Key:     deployKey.Key,
		Created: deployKey.Created,
		Expires: deployKey.Expires,
	}, nil
}

func (r *mutationResolver) AddRepository(ctx context.Context, teamSlug slug.Slug, repoName string) (string, error) {
	actor := authz.ActorFromContext(ctx)
	if _, err := r.database.GetTeamMember(ctx, teamSlug, actor.User.GetID()); errors.Is(err, pgx.ErrNoRows) {
		return "", apierror.ErrUserIsNotTeamMember
	} else if err != nil {
		return "", err
	}

	if err := r.database.AddTeamRepository(ctx, teamSlug, repoName); err != nil {
		return "", err
	}

	if err := r.auditor.TeamAddRepository(ctx, actor.User, teamSlug, repoName); err != nil {
		return "", err
	}

	return repoName, nil
}

func (r *mutationResolver) RemoveRepository(ctx context.Context, teamSlug slug.Slug, repoName string) (string, error) {
	actor := authz.ActorFromContext(ctx)
	if _, err := r.database.GetTeamMember(ctx, teamSlug, actor.User.GetID()); errors.Is(err, pgx.ErrNoRows) {
		return "", apierror.ErrUserIsNotTeamMember
	} else if err != nil {
		return "", err
	}

	if err := r.database.RemoveTeamRepository(ctx, teamSlug, repoName); err != nil {
		return "", err
	}

	if err := r.auditor.TeamRemoveRepository(ctx, actor.User, teamSlug, repoName); err != nil {
		return "", err
	}

	return repoName, nil
}

func (r *queryResolver) Teams(ctx context.Context, offset *int, limit *int, filter *model.TeamsFilter) (*model.TeamList, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireGlobalAuthorization(actor, roles.AuthorizationTeamsList)
	if err != nil {
		return nil, err
	}

	var teams []*database.Team

	p := model.NewPagination(offset, limit)
	var pageInfo model.PageInfo

	if filter != nil && filter.Github != nil {
		teams, err = r.database.GetAllTeamsWithPermissionInGitHubRepo(ctx, filter.Github.RepoName, filter.Github.PermissionName)
		if err != nil {
			return nil, err
		}

		teams, pageInfo = model.PaginatedSlice(teams, p)
	} else {
		var total int
		teams, total, err = r.database.GetTeams(ctx, database.Page{
			Limit:  p.Limit,
			Offset: p.Offset,
		})
		if err != nil {
			return nil, err
		}

		pageInfo = model.NewPageInfo(p, total)
	}

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

func (r *queryResolver) TeamDeleteKey(ctx context.Context, key string) (*model.TeamDeleteKey, error) {
	kid, err := uuid.Parse(key)
	if err != nil {
		return nil, apierror.Errorf("Invalid deletion key: %q", key)
	}

	deleteKey, err := r.database.GetTeamDeleteKey(ctx, kid)
	if err != nil {
		return nil, apierror.Errorf("Unknown deletion key: %q", key)
	}

	actor := authz.ActorFromContext(ctx)
	err = authz.RequireTeamRole(actor, deleteKey.TeamSlug, gensql.RoleNameTeamowner)
	if err != nil {
		return nil, err
	}

	return toGraphTeamDeleteKey(deleteKey), nil
}

func (r *teamResolver) ID(ctx context.Context, obj *model.Team) (*scalar.Ident, error) {
	return ptr.To(scalar.TeamIdent(obj.Slug)), nil
}

func (r *teamResolver) AuditLogs(ctx context.Context, obj *model.Team, offset *int, limit *int) (*model.AuditLogList, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamAuthorization(actor, roles.AuthorizationAuditLogsRead, obj.Slug)
	if err != nil {
		return nil, err
	}

	p := model.NewPagination(offset, limit)
	entries, total, err := r.database.GetAuditLogsForTeam(ctx, obj.Slug, database.Page{
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

func (r *teamResolver) AuditEvents(ctx context.Context, obj *model.Team, offset *int, limit *int, filter *model.AuditEventsFilter) (*auditevent.AuditEventList, error) {
	p := model.NewPagination(offset, limit)

	var entries []*database.AuditEvent
	var total int
	var err error
	var pageInfo model.PageInfo

	if filter != nil && filter.ResourceType != nil {
		entries, total, err = r.database.GetAuditEventsForTeamByResource(ctx, obj.Slug, string(*filter.ResourceType), database.Page{
			Limit:  p.Limit,
			Offset: p.Offset,
		})
		if err != nil {
			return nil, err
		}

		pageInfo = model.NewPageInfo(p, total)
	} else {
		entries, total, err = r.database.GetAuditEventsForTeam(ctx, obj.Slug, database.Page{
			Limit:  p.Limit,
			Offset: p.Offset,
		})
		if err != nil {
			return nil, err
		}

		pageInfo = model.NewPageInfo(p, total)
	}

	nodes, err := toGraphAuditEvents(entries)
	if err != nil {
		return nil, err
	}

	return &auditevent.AuditEventList{
		Nodes:    nodes,
		PageInfo: pageInfo,
	}, nil
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

func (r *teamResolver) SyncErrors(ctx context.Context, obj *model.Team) ([]*model.SyncError, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamAuthorization(actor, roles.AuthorizationTeamsRead, obj.Slug)
	if err != nil {
		return nil, err
	}

	rows, err := r.database.GetTeamReconcilerErrors(ctx, obj.Slug)
	if err != nil {
		return nil, err
	}

	syncErrors := make([]*model.SyncError, 0)
	for _, row := range rows {
		syncErrors = append(syncErrors, &model.SyncError{
			CreatedAt:  row.CreatedAt.Time,
			Reconciler: row.Reconciler,
			Error:      row.ErrorMessage,
		})
	}

	return syncErrors, nil
}

func (r *teamResolver) GithubRepositories(ctx context.Context, obj *model.Team, offset *int, limit *int, filter *model.GitHubRepositoriesFilter) (*model.GitHubRepositoryList, error) {
	page := model.NewPagination(offset, limit)

	state, err := r.database.GetReconcilerStateForTeam(ctx, "github:team", obj.Slug)
	if err != nil {
		return &model.GitHubRepositoryList{
			Nodes: []*model.GitHubRepository{},
		}, nil
	}

	repos, err := toGraphGitHubRepositories(obj.Slug, state, filter)
	if err != nil {
		return nil, err
	}

	nodes, pageInfo := model.PaginatedSlice(repos, page)
	return &model.GitHubRepositoryList{
		Nodes:    nodes,
		PageInfo: pageInfo,
	}, nil
}

func (r *teamResolver) DeletionInProgress(ctx context.Context, obj *model.Team) (bool, error) {
	_, err := r.database.GetActiveTeamBySlug(ctx, obj.Slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}

	return false, err
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

func (r *teamResolver) Status(ctx context.Context, obj *model.Team) (*model.TeamStatus, error) {
	wg := pool.NewWithResults[any]().WithErrors().WithFirstError()

	wg.Go(func() (any, error) {
		apps, err := r.k8sClient.Apps(ctx, obj.Slug.String())
		if err != nil {
			return nil, fmt.Errorf("getting apps from Kubernetes: %w", err)
		}
		failingApps := 0
		for _, app := range apps {
			if app.Status.State == model.StateFailing {
				failingApps++
			}
		}
		return model.AppsStatus{
			Total:   len(apps),
			Failing: failingApps,
		}, nil
	})

	wg.Go(func() (any, error) {
		jobs, err := r.k8sClient.NaisJobs(ctx, obj.Slug.String())
		if err != nil {
			return nil, fmt.Errorf("getting naisjobs from Kubernetes: %w", err)
		}
		failingJobs := 0
		for _, job := range jobs {
			if job.Status.State == model.StateFailing {
				failingJobs++
			}
		}
		return model.JobsStatus{
			Total:   len(jobs),
			Failing: failingJobs,
		}, nil
	})

	wg.Go(func() (any, error) {
		teamEnvs, _, err := r.database.GetTeamEnvironments(ctx, obj.Slug, database.Page{Limit: 50})
		if err != nil {
			return nil, err
		}
		sqlInstances, _, err := r.sqlInstanceClient.SqlInstances(ctx, obj.Slug, teamEnvs)
		failingSqlInstances := 0
		otherConditions := 0
		if err != nil {
			return nil, fmt.Errorf("getting SQL instances from Kubernetes: %w", err)
		}
		for _, sqlInstance := range sqlInstances {
			notReady := sqlInstance.IsNotReady()
			healthy := sqlInstance.IsHealthy()
			if sqlInstance.State != model.SQLInstanceStateRunnable {
				failingSqlInstances++
				continue
			}

			if notReady {
				failingSqlInstances++
			}

			if !notReady && !healthy {
				otherConditions++
			}
		}
		return model.SQLInstancesStatus{
			Total:           len(sqlInstances),
			Failing:         failingSqlInstances,
			OtherConditions: otherConditions,
		}, nil
	})

	res, err := wg.Wait()
	if err != nil {
		return nil, err
	}

	ret := &model.TeamStatus{}

	for _, r := range res {
		switch v := r.(type) {
		case model.AppsStatus:
			ret.Apps = v
		case model.JobsStatus:
			ret.Jobs = v
		case model.SQLInstancesStatus:
			ret.SQLInstances = v
		}
	}

	return ret, nil
}

func (r *teamResolver) SQLInstance(ctx context.Context, obj *model.Team, name string, env string) (*model.SQLInstance, error) {
	return r.sqlInstanceClient.SqlInstance(ctx, env, obj.Slug, name)
}

func (r *teamResolver) SQLInstances(ctx context.Context, obj *model.Team, offset *int, limit *int, orderBy *model.OrderBy) (*model.SQLInstancesList, error) {
	dbEnvs, _, err := r.database.GetTeamEnvironments(ctx, obj.Slug, database.Page{Limit: 50})
	if err != nil {
		return nil, err
	}

	sqlInstances, metrics, err := r.sqlInstanceClient.SqlInstances(ctx, obj.Slug, dbEnvs)
	if err != nil {
		return nil, fmt.Errorf("getting SQL instances from Kubernetes: %w", err)
	}

	if orderBy != nil {
		switch orderBy.Field {
		case model.OrderByFieldName:
			model.SortWith(sqlInstances, func(a, b *model.SQLInstance) bool {
				return model.Compare(a.Name, b.Name, orderBy.Direction)
			})
		case model.OrderByFieldEnv:
			model.SortWith(sqlInstances, func(a, b *model.SQLInstance) bool {
				return model.Compare(a.Env.Name, b.Env.Name, orderBy.Direction)
			})
		case model.OrderByFieldStatus:
			model.SortWith(sqlInstances, func(a, b *model.SQLInstance) bool {
				return model.Compare(strconv.FormatBool(a.IsHealthy()), strconv.FormatBool(b.IsHealthy()), orderBy.Direction)
			})
		case model.OrderByFieldVersion:
			model.SortWith(sqlInstances, func(a, b *model.SQLInstance) bool {
				return model.Compare(a.Type, b.Type, orderBy.Direction)
			})
		case model.OrderByFieldCost:
			model.SortWith(sqlInstances, func(a, b *model.SQLInstance) bool {
				return model.Compare(a.Metrics.Cost, b.Metrics.Cost, orderBy.Direction)
			})
		case model.OrderByFieldCPU:
			model.SortWith(sqlInstances, func(a, b *model.SQLInstance) bool {
				return model.Compare(a.Metrics.CPU.Utilization, b.Metrics.CPU.Utilization, orderBy.Direction)
			})
		case model.OrderByFieldMemory:
			model.SortWith(sqlInstances, func(a, b *model.SQLInstance) bool {
				return model.Compare(a.Metrics.Memory.Utilization, b.Metrics.Memory.Utilization, orderBy.Direction)
			})
		case model.OrderByFieldDisk:
			model.SortWith(sqlInstances, func(a, b *model.SQLInstance) bool {
				return model.Compare(a.Metrics.Disk.Utilization, b.Metrics.Disk.Utilization, orderBy.Direction)
			})
		}
	}
	pagination := model.NewPagination(offset, limit)
	sqlInstances, pageInfo := model.PaginatedSlice(sqlInstances, pagination)

	return &model.SQLInstancesList{
		Nodes:    sqlInstances,
		PageInfo: pageInfo,
		Metrics:  metrics,
	}, nil
}

func (r *teamResolver) Bucket(ctx context.Context, obj *model.Team, name string, env string) (*model.Bucket, error) {
	return r.bucketClient.Bucket(ctx, env, obj.Slug, name)
}

func (r *teamResolver) Buckets(ctx context.Context, obj *model.Team, offset *int, limit *int, orderBy *model.OrderBy) (*model.BucketsList, error) {
	buckets, metrics, err := r.bucketClient.Buckets(ctx, obj.Slug)
	if err != nil {
		return nil, err
	}

	if orderBy != nil {
		switch orderBy.Field {
		case model.OrderByFieldName:
			model.SortWith(buckets, func(a, b *model.Bucket) bool {
				return model.Compare(a.Name, b.Name, orderBy.Direction)
			})
		case model.OrderByFieldEnv:
			model.SortWith(buckets, func(a, b *model.Bucket) bool {
				return model.Compare(a.Env.Name, b.Env.Name, orderBy.Direction)
			})
		}
	}

	pagination := model.NewPagination(offset, limit)
	buckets, pageInfo := model.PaginatedSlice(buckets, pagination)

	return &model.BucketsList{
		Nodes:    buckets,
		PageInfo: pageInfo,
		Metrics:  *metrics,
	}, nil
}

func (r *teamResolver) RedisInstance(ctx context.Context, obj *model.Team, name string, env string) (*model.Redis, error) {
	return r.redisClient.RedisInstance(ctx, env, obj.Slug, name)
}

func (r *teamResolver) Redis(ctx context.Context, obj *model.Team, offset *int, limit *int, orderBy *model.OrderBy) (*model.RedisList, error) {
	redis, metrics, err := r.redisClient.Redis(ctx, obj.Slug)
	if err != nil {
		return nil, err
	}

	if orderBy != nil {
		switch orderBy.Field {
		case model.OrderByFieldName:
			model.SortWith(redis, func(a, b *model.Redis) bool {
				return model.Compare(a.Name, b.Name, orderBy.Direction)
			})
		case model.OrderByFieldEnv:
			model.SortWith(redis, func(a, b *model.Redis) bool {
				return model.Compare(a.Env.Name, b.Env.Name, orderBy.Direction)
			})
		}
	}

	pagination := model.NewPagination(offset, limit)
	redis, pageInfo := model.PaginatedSlice(redis, pagination)

	return &model.RedisList{
		Nodes:    redis,
		Metrics:  *metrics,
		PageInfo: pageInfo,
	}, nil
}

func (r *teamResolver) OpenSearchInstance(ctx context.Context, obj *model.Team, name string, env string) (*model.OpenSearch, error) {
	return r.openSearchClient.OpenSearchInstance(ctx, env, obj.Slug, name)
}

func (r *teamResolver) OpenSearch(ctx context.Context, obj *model.Team, offset *int, limit *int, orderBy *model.OrderBy) (*model.OpenSearchList, error) {
	openSearch, metrics, err := r.openSearchClient.OpenSearch(ctx, obj.Slug)
	if err != nil {
		return nil, err
	}

	if orderBy != nil {
		switch orderBy.Field {
		case model.OrderByFieldName:
			model.SortWith(openSearch, func(a, b *model.OpenSearch) bool {
				return model.Compare(a.Name, b.Name, orderBy.Direction)
			})
		case model.OrderByFieldEnv:
			model.SortWith(openSearch, func(a, b *model.OpenSearch) bool {
				return model.Compare(a.Env.Name, b.Env.Name, orderBy.Direction)
			})
		}
	}

	pagination := model.NewPagination(offset, limit)
	openSearch, pageInfo := model.PaginatedSlice(openSearch, pagination)

	return &model.OpenSearchList{
		Nodes:    openSearch,
		PageInfo: pageInfo,
		Metrics:  *metrics,
	}, nil
}

func (r *teamResolver) KafkaTopic(ctx context.Context, obj *model.Team, name string, env string) (*model.KafkaTopic, error) {
	return r.kafkaClient.Topic(ctx, env, obj.Slug, name)
}

func (r *teamResolver) KafkaTopics(ctx context.Context, obj *model.Team, offset *int, limit *int, orderBy *model.OrderBy) (*model.KafkaTopicList, error) {
	kts, err := r.kafkaClient.Topics(ctx, obj.Slug)
	if err != nil {
		return nil, err
	}

	if orderBy != nil {
		switch orderBy.Field {
		case model.OrderByFieldName:
			model.SortWith(kts, func(a, b *model.KafkaTopic) bool {
				return model.Compare(a.Name, b.Name, orderBy.Direction)
			})
		case model.OrderByFieldEnv:
			model.SortWith(kts, func(a, b *model.KafkaTopic) bool {
				return model.Compare(a.Env.Name, b.Env.Name, orderBy.Direction)
			})
		}
	}

	pagination := model.NewPagination(offset, limit)
	kts, pageInfo := model.PaginatedSlice(kts, pagination)

	return &model.KafkaTopicList{
		Nodes:    kts,
		PageInfo: pageInfo,
	}, nil
}

func (r *teamResolver) BigQuery(ctx context.Context, obj *model.Team, offset *int, limit *int, orderBy *model.OrderBy) (*model.BigQueryDatasetList, error) {
	bqs, err := r.bigQueryDatasetClient.BigQueryDatasets(ctx, obj.Slug)
	if err != nil {
		return nil, err
	}

	if orderBy != nil {
		switch orderBy.Field {
		case model.OrderByFieldName:
			model.SortWith(bqs, func(a, b *model.BigQueryDataset) bool {
				return model.Compare(a.Name, b.Name, orderBy.Direction)
			})
		case model.OrderByFieldEnv:
			model.SortWith(bqs, func(a, b *model.BigQueryDataset) bool {
				return model.Compare(a.Env.Name, b.Env.Name, orderBy.Direction)
			})
		}
	}

	pagination := model.NewPagination(offset, limit)
	bqs, pageInfo := model.PaginatedSlice(bqs, pagination)

	return &model.BigQueryDatasetList{
		Nodes:    bqs,
		PageInfo: pageInfo,
	}, nil
}

func (r *teamResolver) BigQueryDataset(ctx context.Context, obj *model.Team, name string, env string) (*model.BigQueryDataset, error) {
	return r.bigQueryDatasetClient.BigQueryDataset(ctx, env, obj.Slug, name)
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
		case model.OrderByFieldSeverityCritical:
			severities := map[string]int{}
			images := []*model.ImageDetails{}
			for _, app := range apps {
				image, err := r.dependencyTrackClient.GetMetadataForImage(ctx, app.Image)
				if err != nil {
					return nil, fmt.Errorf("getting metadata for image %q: %w", app.Image, err)
				}
				images = append(images, image)
			}

			for _, image := range images {
				if image == nil || image.Summary == nil {
					severities[image.Name] = -1
					continue
				}
				severities[image.Name] = image.Summary.Critical
			}

			model.SortWith(apps, func(a, b *model.App) bool {
				if orderBy.Direction == model.SortOrderAsc {
					if severities[a.Image] == severities[b.Image] {
						return a.Name < b.Name
					}
					return severities[a.Image] < severities[b.Image]
				}
				if severities[a.Image] == severities[b.Image] {
					return a.Name > b.Name
				}
				return severities[a.Image] > severities[b.Image]
			})

		case model.OrderByFieldRiskScore:
			riskScores := map[string]int{}
			images := []*model.ImageDetails{}
			for _, app := range apps {
				image, err := r.dependencyTrackClient.GetMetadataForImage(ctx, app.Image)
				if err != nil {
					return nil, fmt.Errorf("getting metadata for image %q: %w", app.Image, err)
				}
				images = append(images, image)
			}

			for _, image := range images {
				if image == nil || image.Summary == nil {
					riskScores[image.Name] = -1
					continue
				}
				riskScores[image.Name] = image.Summary.RiskScore
			}

			model.SortWith(apps, func(a, b *model.App) bool {
				if orderBy.Direction == model.SortOrderAsc {
					if riskScores[a.Image] == riskScores[b.Image] {
						return a.Name < b.Name
					}
					return riskScores[a.Image] < riskScores[b.Image]
				}
				if riskScores[a.Image] == riskScores[b.Image] {
					return a.Name > b.Name
				}
				return riskScores[a.Image] > riskScores[b.Image]
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
		case model.OrderByFieldSeverityCritical:
			severities := map[string]int{}
			images := []*model.ImageDetails{}
			for _, job := range naisjobs {
				image, err := r.dependencyTrackClient.GetMetadataForImage(ctx, job.Image)
				if err != nil {
					return nil, fmt.Errorf("getting metadata for image %q: %w", job.Image, err)
				}
				images = append(images, image)
			}

			for _, image := range images {
				if image == nil || image.Summary == nil {
					severities[image.Name] = -1
					continue
				}
				severities[image.Name] = image.Summary.RiskScore
			}

			model.SortWith(naisjobs, func(a, b *model.NaisJob) bool {
				if orderBy.Direction == model.SortOrderAsc {
					if severities[a.Image] == severities[b.Image] {
						return a.Name < b.Name
					}
					return severities[a.Image] < severities[b.Image]
				}
				if severities[a.Image] == severities[b.Image] {
					return a.Name > b.Name
				}
				return severities[a.Image] > severities[b.Image]
			})

		case model.OrderByFieldRiskScore:
			riskScores := map[string]int{}
			images := []*model.ImageDetails{}
			for _, job := range naisjobs {
				image, err := r.dependencyTrackClient.GetMetadataForImage(ctx, job.Image)
				if err != nil {
					return nil, fmt.Errorf("getting metadata for image %q: %w", job.Image, err)
				}
				images = append(images, image)
			}

			for _, image := range images {
				if image == nil || image.Summary == nil {
					riskScores[image.Name] = -1
					continue
				}
				riskScores[image.Name] = image.Summary.RiskScore
			}

			model.SortWith(naisjobs, func(a, b *model.NaisJob) bool {
				if orderBy.Direction == model.SortOrderAsc {
					if riskScores[a.Image] == riskScores[b.Image] {
						return a.Name < b.Name
					}
					return riskScores[a.Image] < riskScores[b.Image]
				}
				if riskScores[a.Image] == riskScores[b.Image] {
					return a.Name > b.Name
				}
				return riskScores[a.Image] > riskScores[b.Image]
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

func (r *teamResolver) VulnerabilitiesSummary(ctx context.Context, obj *model.Team) (*model.VulnerabilitySummaryForTeam, error) {
	images, err := r.dependencyTrackClient.GetMetadataForTeam(ctx, obj.Slug.String())
	if err != nil {
		return nil, fmt.Errorf("getting metadata for team %q: %w", obj.Slug.String(), err)
	}

	retVal := &model.VulnerabilitySummaryForTeam{}
	for _, image := range images {
		if image.Summary == nil {
			continue
		}
		if image.Summary.Critical > 0 {
			retVal.Critical += image.Summary.Critical
		}
		if image.Summary.High > 0 {
			retVal.High += image.Summary.High
		}
		if image.Summary.Medium > 0 {
			retVal.Medium += image.Summary.Medium
		}
		if image.Summary.Low > 0 {
			retVal.Low += image.Summary.Low
		}
		if image.Summary.Unassigned > 0 {
			retVal.Unassigned += image.Summary.Unassigned
		}
		if image.Summary.RiskScore > 0 {
			retVal.RiskScore += image.Summary.RiskScore
		}

		for _, ref := range image.GQLVars.WorkloadReferences {
			if ref.Team == obj.Slug.String() {
				retVal.BomCount += 1
			}
		}
	}

	apps, err := r.k8sClient.Apps(ctx, obj.Slug.String())
	if err != nil {
		return nil, fmt.Errorf("getting apps from Kubernetes: %w", err)
	}
	jobs, err := r.k8sClient.NaisJobs(ctx, obj.Slug.String())
	if err != nil {
		return nil, fmt.Errorf("getting naisjobs from Kubernetes: %w", err)
	}

	if len(apps) == 0 && len(jobs) == 0 {
		retVal.Coverage = 0.0
	} else {
		retVal.Coverage = float64(retVal.BomCount) / float64(len(apps)+len(jobs)) * 100
	}

	return retVal, nil
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

func (r *teamResolver) Repositories(ctx context.Context, obj *model.Team, offset *int, limit *int) (*model.RepositoryList, error) {
	page := model.NewPagination(offset, limit)
	auths, err := r.database.ListTeamRepositories(ctx, obj.Slug)
	if err != nil {
		return &model.RepositoryList{
			Nodes: []string{},
		}, nil
	}

	nodes, pageInfo := model.PaginatedSlice(auths, page)
	return &model.RepositoryList{
		Nodes:    nodes,
		PageInfo: pageInfo,
	}, nil
}

func (r *teamDeleteKeyResolver) CreatedBy(ctx context.Context, obj *model.TeamDeleteKey) (*model.User, error) {
	return loader.GetUser(ctx, obj.GQLVars.UserID)
}

func (r *teamDeleteKeyResolver) Team(ctx context.Context, obj *model.TeamDeleteKey) (*model.Team, error) {
	return loader.GetTeam(ctx, obj.GQLVars.TeamSlug)
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

func (r *teamMemberResolver) Reconcilers(ctx context.Context, obj *model.TeamMember) ([]*model.TeamMemberReconciler, error) {
	uid, err := obj.UserID.AsUUID()
	if err != nil {
		return nil, err
	}

	rows, err := r.database.GetTeamMemberOptOuts(ctx, uid, obj.TeamSlug)
	if err != nil {
		return nil, err
	}
	return toGraphTeamMemberReconcilers(rows), nil
}

func (r *teamMemberReconcilerResolver) Reconciler(ctx context.Context, obj *model.TeamMemberReconciler) (*model.Reconciler, error) {
	reconciler, err := r.database.GetReconciler(ctx, obj.GQLVars.Name)
	if err != nil {
		return nil, err
	}

	return toGraphReconciler(reconciler), nil
}

func (r *Resolver) Team() gengql.TeamResolver { return &teamResolver{r} }

func (r *Resolver) TeamDeleteKey() gengql.TeamDeleteKeyResolver { return &teamDeleteKeyResolver{r} }

func (r *Resolver) TeamMember() gengql.TeamMemberResolver { return &teamMemberResolver{r} }

func (r *Resolver) TeamMemberReconciler() gengql.TeamMemberReconcilerResolver {
	return &teamMemberReconcilerResolver{r}
}

type (
	teamResolver                 struct{ *Resolver }
	teamDeleteKeyResolver        struct{ *Resolver }
	teamMemberResolver           struct{ *Resolver }
	teamMemberReconcilerResolver struct{ *Resolver }
)

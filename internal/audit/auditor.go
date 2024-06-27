package audit

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/model/auditevent"
	"github.com/nais/api/internal/slug"
)

// Auditor persists audit events to the database.
type Auditor struct {
	db database.AuditEventsRepo
}

func NewAuditor(db database.Database) *Auditor {
	return &Auditor{db: db}
}

func (a *Auditor) TeamMemberAdded(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, memberEmail string, role model.TeamRole) error {
	return a.db.CreateAuditEvent(ctx, auditevent.AuditEventMemberAdded{
		BaseTeamAuditEvent: baseTeamAuditEvent(
			actor,
			team,
			model.AuditEventActionTeamMemberAdded,
			model.AuditEventResourceTypeTeamMember,
			team.String(),
		),
		Data: model.AuditEventMemberAddedData{
			MemberEmail: memberEmail,
			Role:        role,
		},
	})
}

func (a *Auditor) TeamMemberRemoved(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, memberEmail string) error {
	return a.db.CreateAuditEvent(ctx, auditevent.AuditEventMemberRemoved{
		BaseTeamAuditEvent: baseTeamAuditEvent(
			actor,
			team,
			model.AuditEventActionTeamMemberRemoved,
			model.AuditEventResourceTypeTeamMember,
			team.String(),
		),
		Data: model.AuditEventMemberRemovedData{
			MemberEmail: memberEmail,
		},
	})
}

func (a *Auditor) TeamMemberSetRole(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, memberEmail string, role model.TeamRole) error {
	return a.db.CreateAuditEvent(ctx, auditevent.AuditEventMemberSetRole{
		BaseTeamAuditEvent: baseTeamAuditEvent(
			actor,
			team,
			model.AuditEventActionTeamMemberSetRole,
			model.AuditEventResourceTypeTeamMember,
			team.String(),
		),
		Data: model.AuditEventMemberSetRoleData{
			MemberEmail: memberEmail,
			Role:        role,
		},
	})
}

func (a *Auditor) TeamCreated(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug) error {
	return a.db.CreateAuditEvent(ctx, baseTeamAuditEvent(
		actor,
		team,
		model.AuditEventActionTeamCreated,
		model.AuditEventResourceTypeTeam,
		team.String(),
	))
}

func (a *Auditor) TeamDeletionConfirmed(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug) error {
	return a.db.CreateAuditEvent(ctx, baseTeamAuditEvent(
		actor,
		team,
		model.AuditEventActionTeamDeletionConfirmed,
		model.AuditEventResourceTypeTeam,
		team.String(),
	))
}

func (a *Auditor) TeamDeletionRequested(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug) error {
	return a.db.CreateAuditEvent(ctx, baseTeamAuditEvent(
		actor,
		team,
		model.AuditEventActionTeamDeletionRequested,
		model.AuditEventResourceTypeTeam,
		team.String(),
	))
}

func (a *Auditor) TeamRotatedDeployKey(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug) error {
	return a.db.CreateAuditEvent(ctx, baseTeamAuditEvent(
		actor,
		team,
		model.AuditEventActionTeamDeployKeyRotated,
		model.AuditEventResourceTypeTeam,
		team.String(),
	))
}

func (a *Auditor) TeamSynchronized(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug) error {
	return a.db.CreateAuditEvent(ctx, baseTeamAuditEvent(
		actor,
		team,
		model.AuditEventActionTeamSynchronized,
		model.AuditEventResourceTypeTeam,
		team.String(),
	))
}

func (a *Auditor) TeamSetPurpose(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, purpose string) error {
	return a.db.CreateAuditEvent(ctx, auditevent.AuditEventTeamSetPurpose{
		BaseTeamAuditEvent: baseTeamAuditEvent(
			actor,
			team,
			model.AuditEventActionTeamSetPurpose,
			model.AuditEventResourceTypeTeam,
			team.String(),
		), Data: model.AuditEventTeamSetPurposeData{
			Purpose: purpose,
		},
	})
}

func (a *Auditor) TeamSetDefaultSlackChannel(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, defaultSlackChannel string) error {
	return a.db.CreateAuditEvent(ctx, auditevent.AuditEventTeamSetDefaultSlackChannel{
		BaseTeamAuditEvent: baseTeamAuditEvent(
			actor,
			team,
			model.AuditEventActionTeamSetDefaultSLACkChannel,
			model.AuditEventResourceTypeTeam,
			team.String(),
		),
		Data: model.AuditEventTeamSetDefaultSlackChannelData{
			DefaultSlackChannel: defaultSlackChannel,
		},
	})
}

func (a *Auditor) TeamSetAlertsSlackChannel(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, environment, channelName string) error {
	return a.db.CreateAuditEvent(ctx, auditevent.AuditEventTeamSetAlertsSlackChannel{
		BaseTeamAuditEvent: baseTeamAuditEvent(
			actor,
			team,
			model.AuditEventActionTeamSetAlertsSLACkChannel,
			model.AuditEventResourceTypeTeam,
			team.String(),
		),
		Data: model.AuditEventTeamSetAlertsSlackChannelData{
			Environment: environment,
			ChannelName: channelName,
		},
	})
}

func (a *Auditor) TeamAddRepository(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, repositoryName string) error {
	return a.db.CreateAuditEvent(ctx, auditevent.AuditEventTeamAddRepository{
		BaseTeamAuditEvent: baseTeamAuditEvent(
			actor,
			team,
			model.AuditEventActionAdded,
			model.AuditEventResourceTypeTeamRepository,
			team.String(),
		),
		Data: model.AuditEventTeamAddRepositoryData{
			RepositoryName: repositoryName,
		},
	})
}

func (a *Auditor) TeamRemoveRepository(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, repositoryName string) error {
	return a.db.CreateAuditEvent(ctx, auditevent.AuditEventTeamRemoveRepository{
		BaseTeamAuditEvent: baseTeamAuditEvent(
			actor,
			team,
			model.AuditEventActionRemoved,
			model.AuditEventResourceTypeTeamRepository,
			team.String(),
		),
		Data: model.AuditEventTeamRemoveRepositoryData{
			RepositoryName: repositoryName,
		},
	})
}

func baseTeamAuditEvent(
	actor authz.AuthenticatedUser,
	team slug.Slug,
	action model.AuditEventAction,
	resourceType model.AuditEventResourceType,
	resourceName string,
) auditevent.BaseTeamAuditEvent {
	return auditevent.BaseTeamAuditEvent{
		BaseAuditEvent: auditevent.BaseAuditEvent{
			Action:       action,
			Actor:        actor.Identity(),
			ResourceType: resourceType,
			ResourceName: resourceName,
		},
		GQLVars: auditevent.BaseTeamAuditEventGQLVars{
			Team: team,
		},
	}
}

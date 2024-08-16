package audit

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
)

// Auditor mediates persistence of audit events.
type Auditor struct {
	db database.AuditEventsRepo
}

func NewAuditor(db database.Database) *Auditor {
	return &Auditor{db: db}
}

func (a *Auditor) AppDeleted(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, env, applicationName string) error {
	return a.db.CreateAuditEvent(ctx, BaseAuditEvent{
		Action:       model.AuditEventActionDeleted,
		Actor:        actor.Identity(),
		ResourceType: model.AuditEventResourceTypeApp,
		ResourceName: applicationName,
		GQLVars: BaseAuditEventGQLVars{
			Team:        team,
			Environment: env,
		},
	})
}

func (a *Auditor) AppRestarted(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, env, applicationName string) error {
	return a.db.CreateAuditEvent(ctx, BaseAuditEvent{
		Action:       model.AuditEventActionRestarted,
		Actor:        actor.Identity(),
		ResourceType: model.AuditEventResourceTypeApp,
		ResourceName: applicationName,
		GQLVars: BaseAuditEventGQLVars{
			Team:        team,
			Environment: env,
		},
	})
}

func (a *Auditor) NaisjobDeleted(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, env, jobName string) error {
	return a.db.CreateAuditEvent(ctx, BaseAuditEvent{
		Action:       model.AuditEventActionDeleted,
		Actor:        actor.Identity(),
		ResourceType: model.AuditEventResourceTypeNaisjob,
		ResourceName: jobName,
		GQLVars: BaseAuditEventGQLVars{
			Team:        team,
			Environment: env,
		},
	})
}

func (a *Auditor) SecretCreated(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, env, secretName string) error {
	return a.db.CreateAuditEvent(ctx, BaseAuditEvent{
		Action:       model.AuditEventActionCreated,
		Actor:        actor.Identity(),
		ResourceType: model.AuditEventResourceTypeSecret,
		ResourceName: secretName,
		GQLVars: BaseAuditEventGQLVars{
			Team:        team,
			Environment: env,
		},
	})
}

func (a *Auditor) SecretUpdated(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, env, secretName string) error {
	return a.db.CreateAuditEvent(ctx, BaseAuditEvent{
		Action:       model.AuditEventActionUpdated,
		Actor:        actor.Identity(),
		ResourceType: model.AuditEventResourceTypeSecret,
		ResourceName: secretName,
		GQLVars: BaseAuditEventGQLVars{
			Team:        team,
			Environment: env,
		},
	})
}

func (a *Auditor) SecretDeleted(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, env, secretName string) error {
	return a.db.CreateAuditEvent(ctx, BaseAuditEvent{
		Action:       model.AuditEventActionDeleted,
		Actor:        actor.Identity(),
		ResourceType: model.AuditEventResourceTypeSecret,
		ResourceName: secretName,
		GQLVars: BaseAuditEventGQLVars{
			Team:        team,
			Environment: env,
		},
	})
}

func (a *Auditor) TeamMemberAdded(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, memberEmail string, role model.TeamRole) error {
	return a.db.CreateAuditEvent(ctx, AuditEventMemberAdded{
		BaseAuditEvent: BaseAuditEvent{
			Action:       model.AuditEventActionAdded,
			Actor:        actor.Identity(),
			ResourceType: model.AuditEventResourceTypeTeamMember,
			ResourceName: team.String(),
			GQLVars: BaseAuditEventGQLVars{
				Team: team,
			},
		},
		Data: model.AuditEventMemberAddedData{
			MemberEmail: memberEmail,
			Role:        role,
		},
	})
}

func (a *Auditor) TeamMemberRemoved(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, memberEmail string) error {
	return a.db.CreateAuditEvent(ctx, AuditEventMemberRemoved{
		BaseAuditEvent: BaseAuditEvent{
			Action:       model.AuditEventActionRemoved,
			Actor:        actor.Identity(),
			ResourceType: model.AuditEventResourceTypeTeamMember,
			ResourceName: team.String(),
			GQLVars: BaseAuditEventGQLVars{
				Team: team,
			},
		},
		Data: model.AuditEventMemberRemovedData{
			MemberEmail: memberEmail,
		},
	})
}

func (a *Auditor) TeamMemberSetRole(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, memberEmail string, role model.TeamRole) error {
	return a.db.CreateAuditEvent(ctx, AuditEventMemberSetRole{
		BaseAuditEvent: BaseAuditEvent{
			Action:       model.AuditEventActionTeamMemberSetRole,
			Actor:        actor.Identity(),
			ResourceType: model.AuditEventResourceTypeTeamMember,
			ResourceName: team.String(),
			GQLVars: BaseAuditEventGQLVars{
				Team: team,
			},
		},
		Data: model.AuditEventMemberSetRoleData{
			MemberEmail: memberEmail,
			Role:        role,
		},
	})
}

func (a *Auditor) TeamCreated(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug) error {
	return a.db.CreateAuditEvent(ctx, BaseAuditEvent{
		Action:       model.AuditEventActionCreated,
		Actor:        actor.Identity(),
		ResourceType: model.AuditEventResourceTypeTeam,
		ResourceName: team.String(),
		GQLVars: BaseAuditEventGQLVars{
			Team: team,
		},
	})
}

func (a *Auditor) TeamDeletionConfirmed(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug) error {
	return a.db.CreateAuditEvent(ctx, BaseAuditEvent{
		Action:       model.AuditEventActionTeamDeletionConfirmed,
		Actor:        actor.Identity(),
		ResourceType: model.AuditEventResourceTypeTeam,
		ResourceName: team.String(),
		GQLVars: BaseAuditEventGQLVars{
			Team: team,
		},
	})
}

func (a *Auditor) TeamDeletionRequested(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug) error {
	return a.db.CreateAuditEvent(ctx, BaseAuditEvent{
		Action:       model.AuditEventActionTeamDeletionRequested,
		Actor:        actor.Identity(),
		ResourceType: model.AuditEventResourceTypeTeam,
		ResourceName: team.String(),
		GQLVars: BaseAuditEventGQLVars{
			Team: team,
		},
	})
}

func (a *Auditor) TeamRotatedDeployKey(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug) error {
	return a.db.CreateAuditEvent(ctx, BaseAuditEvent{
		Action:       model.AuditEventActionTeamDeployKeyRotated,
		Actor:        actor.Identity(),
		ResourceType: model.AuditEventResourceTypeTeam,
		ResourceName: team.String(),
		GQLVars: BaseAuditEventGQLVars{
			Team: team,
		},
	})
}

func (a *Auditor) TeamSynchronized(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug) error {
	return a.db.CreateAuditEvent(ctx, BaseAuditEvent{
		Action:       model.AuditEventActionSynchronized,
		Actor:        actor.Identity(),
		ResourceType: model.AuditEventResourceTypeTeam,
		ResourceName: team.String(),
		GQLVars: BaseAuditEventGQLVars{
			Team: team,
		},
	})
}

func (a *Auditor) TeamSetPurpose(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, purpose string) error {
	return a.db.CreateAuditEvent(ctx, AuditEventTeamSetPurpose{
		BaseAuditEvent: BaseAuditEvent{
			Action:       model.AuditEventActionTeamSetPurpose,
			Actor:        actor.Identity(),
			ResourceType: model.AuditEventResourceTypeTeam,
			ResourceName: team.String(),
			GQLVars: BaseAuditEventGQLVars{
				Team: team,
			},
		},
		Data: model.AuditEventTeamSetPurposeData{
			Purpose: purpose,
		},
	})
}

func (a *Auditor) TeamSetDefaultSlackChannel(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, defaultSlackChannel string) error {
	return a.db.CreateAuditEvent(ctx, AuditEventTeamSetDefaultSlackChannel{
		BaseAuditEvent: BaseAuditEvent{
			Action:       model.AuditEventActionTeamSetDefaultSLACkChannel,
			Actor:        actor.Identity(),
			ResourceType: model.AuditEventResourceTypeTeam,
			ResourceName: team.String(),
			GQLVars: BaseAuditEventGQLVars{
				Team: team,
			},
		},
		Data: model.AuditEventTeamSetDefaultSlackChannelData{
			DefaultSlackChannel: defaultSlackChannel,
		},
	})
}

func (a *Auditor) TeamSetAlertsSlackChannel(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, environment, channelName string) error {
	return a.db.CreateAuditEvent(ctx, AuditEventTeamSetAlertsSlackChannel{
		BaseAuditEvent: BaseAuditEvent{
			Action:       model.AuditEventActionTeamSetAlertsSLACkChannel,
			Actor:        actor.Identity(),
			ResourceType: model.AuditEventResourceTypeTeam,
			ResourceName: team.String(),
			GQLVars: BaseAuditEventGQLVars{
				Team: team,
			},
		},
		Data: model.AuditEventTeamSetAlertsSlackChannelData{
			Environment: environment,
			ChannelName: channelName,
		},
	})
}

func (a *Auditor) TeamAddRepository(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, repositoryName string) error {
	return a.db.CreateAuditEvent(ctx, AuditEventTeamAddRepository{
		BaseAuditEvent: BaseAuditEvent{
			Action:       model.AuditEventActionAdded,
			Actor:        actor.Identity(),
			ResourceType: model.AuditEventResourceTypeTeamRepository,
			ResourceName: team.String(),
			GQLVars: BaseAuditEventGQLVars{
				Team: team,
			},
		},
		Data: model.AuditEventTeamAddRepositoryData{
			RepositoryName: repositoryName,
		},
	})
}

func (a *Auditor) TeamRemoveRepository(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, repositoryName string) error {
	return a.db.CreateAuditEvent(ctx, AuditEventTeamRemoveRepository{
		BaseAuditEvent: BaseAuditEvent{
			Action:       model.AuditEventActionRemoved,
			Actor:        actor.Identity(),
			ResourceType: model.AuditEventResourceTypeTeamRepository,
			ResourceName: team.String(),
			GQLVars: BaseAuditEventGQLVars{
				Team: team,
			},
		},
		Data: model.AuditEventTeamRemoveRepositoryData{
			RepositoryName: repositoryName,
		},
	})
}

func (a *Auditor) UnleashCreated(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, unleashName string) error {
	return a.db.CreateAuditEvent(ctx, BaseAuditEvent{
		Action:       model.AuditEventActionCreated,
		Actor:        actor.Identity(),
		ResourceType: model.AuditEventResourceTypeUnleash,
		ResourceName: unleashName,
		GQLVars: BaseAuditEventGQLVars{
			Team: team,
		},
	})
}

func (a *Auditor) UnleashUpdated(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, unleashName string) error {
	return a.db.CreateAuditEvent(ctx, BaseAuditEvent{
		Action:       model.AuditEventActionUpdated,
		Actor:        actor.Identity(),
		ResourceType: model.AuditEventResourceTypeUnleash,
		ResourceName: unleashName,
		GQLVars: BaseAuditEventGQLVars{
			Team: team,
		},
	})
}

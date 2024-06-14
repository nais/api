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
	return a.db.CreateAuditEvent(ctx, auditevent.NewAuditEventMemberAdded(
		baseAuditEvent(
			actor,
			team,
			model.AuditEventActionTeamMemberAdded,
			model.AuditEventResourceTypeTeamMember,
			team.String(),
		),
		auditevent.AuditEventMemberAddedData{
			MemberEmail: memberEmail,
			Role:        role,
		}))
}

func (a *Auditor) TeamMemberRemoved(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, memberEmail string) error {
	return a.db.CreateAuditEvent(ctx, auditevent.NewAuditEventMemberRemoved(
		baseAuditEvent(
			actor,
			team,
			model.AuditEventActionTeamMemberRemoved,
			model.AuditEventResourceTypeTeamMember,
			team.String(),
		),
		auditevent.AuditEventMemberRemovedData{
			MemberEmail: memberEmail,
		}))
}

func (a *Auditor) TeamMemberSetRole(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, memberEmail string, role model.TeamRole) error {
	return a.db.CreateAuditEvent(ctx, auditevent.NewAuditEventMemberSetRole(baseAuditEvent(
		actor,
		team,
		model.AuditEventActionTeamMemberSetRole,
		model.AuditEventResourceTypeTeamMember,
		team.String(),
	), auditevent.AuditEventMemberSetRoleData{
		MemberEmail: memberEmail,
		Role:        role,
	}))
}

func (a *Auditor) TeamCreated(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug) error {
	return a.db.CreateAuditEvent(ctx, baseAuditEvent(
		actor,
		team,
		model.AuditEventActionTeamCreated,
		model.AuditEventResourceTypeTeam,
		team.String(),
	))
}

func (a *Auditor) TeamDeletionConfirmed(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug) error {
	return a.db.CreateAuditEvent(ctx, baseAuditEvent(
		actor,
		team,
		model.AuditEventActionTeamDeletionConfirmed,
		model.AuditEventResourceTypeTeam,
		team.String(),
	))
}

func (a *Auditor) TeamDeletionRequested(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug) error {
	return a.db.CreateAuditEvent(ctx, baseAuditEvent(
		actor,
		team,
		model.AuditEventActionTeamDeletionRequested,
		model.AuditEventResourceTypeTeam,
		team.String(),
	))
}

func (a *Auditor) TeamRotatedDeployKey(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug) error {
	return a.db.CreateAuditEvent(ctx, baseAuditEvent(
		actor,
		team,
		model.AuditEventActionTeamDeployKeyRotated,
		model.AuditEventResourceTypeTeam,
		team.String(),
	))
}

func (a *Auditor) TeamSynchronized(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug) error {
	return a.db.CreateAuditEvent(ctx, baseAuditEvent(
		actor,
		team,
		model.AuditEventActionTeamSynchronized,
		model.AuditEventResourceTypeTeam,
		team.String(),
	))
}

func (a *Auditor) TeamSetPurpose(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, purpose string) error {
	return a.db.CreateAuditEvent(ctx, auditevent.NewAuditEventTeamSetPurpose(baseAuditEvent(
		actor,
		team,
		model.AuditEventActionTeamSetPurpose,
		model.AuditEventResourceTypeTeam,
		team.String(),
	), auditevent.AuditEventTeamSetPurposeData{
		Purpose: purpose,
	}))
}

func (a *Auditor) TeamSetDefaultSlackChannel(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, defaultSlackChannel string) error {
	return a.db.CreateAuditEvent(ctx, auditevent.NewAuditEventTeamSetDefaultSlackChannel(baseAuditEvent(
		actor,
		team,
		model.AuditEventActionTeamSetDefaultSLACkChannel,
		model.AuditEventResourceTypeTeam,
		team.String(),
	), auditevent.AuditEventTeamSetDefaultSlackChannelData{
		DefaultSlackChannel: defaultSlackChannel,
	}))
}

func (a *Auditor) TeamSetAlertsSlackChannel(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, environment, channelName string) error {
	return a.db.CreateAuditEvent(ctx, auditevent.NewAuditEventTeamSetAlertsSlackChannel(baseAuditEvent(
		actor,
		team,
		model.AuditEventActionTeamSetAlertsSLACkChannel,
		model.AuditEventResourceTypeTeam,
		team.String(),
	), auditevent.AuditEventTeamSetAlertsSlackChannelData{
		Environment: environment,
		ChannelName: channelName,
	}))
}

func baseAuditEvent(
	actor authz.AuthenticatedUser,
	team slug.Slug,
	action model.AuditEventAction,
	resourceType model.AuditEventResourceType,
	resourceName string,
) auditevent.BaseAuditEvent {
	return auditevent.BaseAuditEvent{
		Action:       action,
		Actor:        actor.Identity(),
		ResourceType: resourceType,
		ResourceName: resourceName,
		Team:         team,
	}
}

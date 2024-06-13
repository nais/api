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
			model.AuditEventResourceTypeTeamMember,
			model.AuditEventActionTeamMemberAdded,
			actor,
			team,
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
			model.AuditEventResourceTypeTeamMember,
			model.AuditEventActionTeamMemberRemoved,
			actor,
			team,
			team.String(),
		),
		auditevent.AuditEventMemberRemovedData{
			MemberEmail: memberEmail,
		}))
}

func (a *Auditor) TeamMemberSetRole(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, memberEmail string, role model.TeamRole) error {
	return a.db.CreateAuditEvent(ctx, auditevent.NewAuditEventMemberSetRole(baseAuditEvent(
		model.AuditEventResourceTypeTeamMember,
		model.AuditEventActionTeamMemberSetRole,
		actor,
		team,
		team.String(),
	), auditevent.AuditEventMemberSetRoleData{
		MemberEmail: memberEmail,
		Role:        role,
	}))
}

func (a *Auditor) TeamCreated(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug) error {
	return a.db.CreateAuditEvent(ctx, baseAuditEvent(
		model.AuditEventResourceTypeTeam,
		model.AuditEventActionTeamCreated,
		actor,
		team,
		team.String(),
	))
}

func (a *Auditor) TeamDeletionConfirmed(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug) error {
	return a.db.CreateAuditEvent(ctx, baseAuditEvent(
		model.AuditEventResourceTypeTeam,
		model.AuditEventActionTeamDeletionConfirmed,
		actor,
		team,
		team.String(),
	))
}

func (a *Auditor) TeamDeletionRequested(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug) error {
	return a.db.CreateAuditEvent(ctx, baseAuditEvent(
		model.AuditEventResourceTypeTeam,
		model.AuditEventActionTeamDeletionRequested,
		actor,
		team,
		team.String(),
	))
}

func (a *Auditor) TeamRotatedDeployKey(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug) error {
	return a.db.CreateAuditEvent(ctx, baseAuditEvent(
		model.AuditEventResourceTypeTeam,
		model.AuditEventActionTeamDeployKeyRotated,
		actor,
		team,
		team.String(),
	))
}

func (a *Auditor) TeamSynchronized(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug) error {
	return a.db.CreateAuditEvent(ctx, baseAuditEvent(
		model.AuditEventResourceTypeTeam,
		model.AuditEventActionTeamSynchronized,
		actor,
		team,
		team.String(),
	))
}

func (a *Auditor) TeamUpdated(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug) error {
	return a.db.CreateAuditEvent(ctx, baseAuditEvent(
		model.AuditEventResourceTypeTeam,
		model.AuditEventActionTeamUpdated,
		actor,
		team,
		team.String(),
	))
}

func baseAuditEvent(
	resourceType model.AuditEventResourceType,
	action model.AuditEventAction,
	actor authz.AuthenticatedUser,
	team slug.Slug,
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

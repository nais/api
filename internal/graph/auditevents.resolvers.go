package graph

import (
	"context"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/model/auditevent"
)

func (r *auditEventMemberAddedResolver) Team(ctx context.Context, obj *auditevent.AuditEventMemberAdded) (*model.Team, error) {
	return loader.GetTeam(ctx, obj.GQLVars.Team)
}

func (r *auditEventMemberRemovedResolver) Team(ctx context.Context, obj *auditevent.AuditEventMemberRemoved) (*model.Team, error) {
	return loader.GetTeam(ctx, obj.GQLVars.Team)
}

func (r *auditEventMemberSetRoleResolver) Team(ctx context.Context, obj *auditevent.AuditEventMemberSetRole) (*model.Team, error) {
	return loader.GetTeam(ctx, obj.GQLVars.Team)
}

func (r *auditEventTeamSetAlertsSlackChannelResolver) Team(ctx context.Context, obj *auditevent.AuditEventTeamSetAlertsSlackChannel) (*model.Team, error) {
	return loader.GetTeam(ctx, obj.GQLVars.Team)
}

func (r *auditEventTeamSetDefaultSlackChannelResolver) Team(ctx context.Context, obj *auditevent.AuditEventTeamSetDefaultSlackChannel) (*model.Team, error) {
	return loader.GetTeam(ctx, obj.GQLVars.Team)
}

func (r *auditEventTeamSetPurposeResolver) Team(ctx context.Context, obj *auditevent.AuditEventTeamSetPurpose) (*model.Team, error) {
	return loader.GetTeam(ctx, obj.GQLVars.Team)
}

func (r *baseAuditEventResolver) Team(ctx context.Context, obj *auditevent.BaseAuditEvent) (*model.Team, error) {
	return nil, nil
}

func (r *baseTeamAuditEventResolver) Team(ctx context.Context, obj *auditevent.BaseTeamAuditEvent) (*model.Team, error) {
	return loader.GetTeam(ctx, obj.GQLVars.Team)
}

func (r *Resolver) AuditEventMemberAdded() gengql.AuditEventMemberAddedResolver {
	return &auditEventMemberAddedResolver{r}
}

func (r *Resolver) AuditEventMemberRemoved() gengql.AuditEventMemberRemovedResolver {
	return &auditEventMemberRemovedResolver{r}
}

func (r *Resolver) AuditEventMemberSetRole() gengql.AuditEventMemberSetRoleResolver {
	return &auditEventMemberSetRoleResolver{r}
}

func (r *Resolver) AuditEventTeamSetAlertsSlackChannel() gengql.AuditEventTeamSetAlertsSlackChannelResolver {
	return &auditEventTeamSetAlertsSlackChannelResolver{r}
}

func (r *Resolver) AuditEventTeamSetDefaultSlackChannel() gengql.AuditEventTeamSetDefaultSlackChannelResolver {
	return &auditEventTeamSetDefaultSlackChannelResolver{r}
}

func (r *Resolver) AuditEventTeamSetPurpose() gengql.AuditEventTeamSetPurposeResolver {
	return &auditEventTeamSetPurposeResolver{r}
}

func (r *Resolver) BaseAuditEvent() gengql.BaseAuditEventResolver { return &baseAuditEventResolver{r} }

func (r *Resolver) BaseTeamAuditEvent() gengql.BaseTeamAuditEventResolver {
	return &baseTeamAuditEventResolver{r}
}

type (
	auditEventMemberAddedResolver                struct{ *Resolver }
	auditEventMemberRemovedResolver              struct{ *Resolver }
	auditEventMemberSetRoleResolver              struct{ *Resolver }
	auditEventTeamSetAlertsSlackChannelResolver  struct{ *Resolver }
	auditEventTeamSetDefaultSlackChannelResolver struct{ *Resolver }
	auditEventTeamSetPurposeResolver             struct{ *Resolver }
	baseAuditEventResolver                       struct{ *Resolver }
	baseTeamAuditEventResolver                   struct{ *Resolver }
)

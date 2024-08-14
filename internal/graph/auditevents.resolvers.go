package graph

import (
	"context"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/model/auditevent"
)

func (r *auditEventMemberAddedResolver) Team(ctx context.Context, obj *auditevent.AuditEventMemberAdded) (*model.Team, error) {
	return resolveEventTeam(ctx, obj.BaseAuditEvent)
}

func (r *auditEventMemberAddedResolver) Env(ctx context.Context, obj *auditevent.AuditEventMemberAdded) (*model.Env, error) {
	return resolveEventEnv(ctx, obj.BaseAuditEvent)
}

func (r *auditEventMemberRemovedResolver) Team(ctx context.Context, obj *auditevent.AuditEventMemberRemoved) (*model.Team, error) {
	return resolveEventTeam(ctx, obj.BaseAuditEvent)
}

func (r *auditEventMemberRemovedResolver) Env(ctx context.Context, obj *auditevent.AuditEventMemberRemoved) (*model.Env, error) {
	return resolveEventEnv(ctx, obj.BaseAuditEvent)
}

func (r *auditEventMemberSetRoleResolver) Team(ctx context.Context, obj *auditevent.AuditEventMemberSetRole) (*model.Team, error) {
	return resolveEventTeam(ctx, obj.BaseAuditEvent)
}

func (r *auditEventMemberSetRoleResolver) Env(ctx context.Context, obj *auditevent.AuditEventMemberSetRole) (*model.Env, error) {
	return resolveEventEnv(ctx, obj.BaseAuditEvent)
}

func (r *auditEventTeamAddRepositoryResolver) Team(ctx context.Context, obj *auditevent.AuditEventTeamAddRepository) (*model.Team, error) {
	return resolveEventTeam(ctx, obj.BaseAuditEvent)
}

func (r *auditEventTeamAddRepositoryResolver) Env(ctx context.Context, obj *auditevent.AuditEventTeamAddRepository) (*model.Env, error) {
	return resolveEventEnv(ctx, obj.BaseAuditEvent)
}

func (r *auditEventTeamRemoveRepositoryResolver) Team(ctx context.Context, obj *auditevent.AuditEventTeamRemoveRepository) (*model.Team, error) {
	return resolveEventTeam(ctx, obj.BaseAuditEvent)
}

func (r *auditEventTeamRemoveRepositoryResolver) Env(ctx context.Context, obj *auditevent.AuditEventTeamRemoveRepository) (*model.Env, error) {
	return resolveEventEnv(ctx, obj.BaseAuditEvent)
}

func (r *auditEventTeamSetAlertsSlackChannelResolver) Team(ctx context.Context, obj *auditevent.AuditEventTeamSetAlertsSlackChannel) (*model.Team, error) {
	return resolveEventTeam(ctx, obj.BaseAuditEvent)
}

func (r *auditEventTeamSetAlertsSlackChannelResolver) Env(ctx context.Context, obj *auditevent.AuditEventTeamSetAlertsSlackChannel) (*model.Env, error) {
	return resolveEventEnv(ctx, obj.BaseAuditEvent)
}

func (r *auditEventTeamSetDefaultSlackChannelResolver) Team(ctx context.Context, obj *auditevent.AuditEventTeamSetDefaultSlackChannel) (*model.Team, error) {
	return resolveEventTeam(ctx, obj.BaseAuditEvent)
}

func (r *auditEventTeamSetDefaultSlackChannelResolver) Env(ctx context.Context, obj *auditevent.AuditEventTeamSetDefaultSlackChannel) (*model.Env, error) {
	return resolveEventEnv(ctx, obj.BaseAuditEvent)
}

func (r *auditEventTeamSetPurposeResolver) Team(ctx context.Context, obj *auditevent.AuditEventTeamSetPurpose) (*model.Team, error) {
	return resolveEventTeam(ctx, obj.BaseAuditEvent)
}

func (r *auditEventTeamSetPurposeResolver) Env(ctx context.Context, obj *auditevent.AuditEventTeamSetPurpose) (*model.Env, error) {
	return resolveEventEnv(ctx, obj.BaseAuditEvent)
}

func (r *baseAuditEventResolver) Team(ctx context.Context, obj *auditevent.BaseAuditEvent) (*model.Team, error) {
	return resolveEventTeam(ctx, *obj)
}

func (r *baseAuditEventResolver) Env(ctx context.Context, obj *auditevent.BaseAuditEvent) (*model.Env, error) {
	return resolveEventEnv(ctx, *obj)
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

func (r *Resolver) AuditEventTeamAddRepository() gengql.AuditEventTeamAddRepositoryResolver {
	return &auditEventTeamAddRepositoryResolver{r}
}

func (r *Resolver) AuditEventTeamRemoveRepository() gengql.AuditEventTeamRemoveRepositoryResolver {
	return &auditEventTeamRemoveRepositoryResolver{r}
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

type (
	auditEventMemberAddedResolver                struct{ *Resolver }
	auditEventMemberRemovedResolver              struct{ *Resolver }
	auditEventMemberSetRoleResolver              struct{ *Resolver }
	auditEventTeamAddRepositoryResolver          struct{ *Resolver }
	auditEventTeamRemoveRepositoryResolver       struct{ *Resolver }
	auditEventTeamSetAlertsSlackChannelResolver  struct{ *Resolver }
	auditEventTeamSetDefaultSlackChannelResolver struct{ *Resolver }
	auditEventTeamSetPurposeResolver             struct{ *Resolver }
	baseAuditEventResolver                       struct{ *Resolver }
)

package graph

import (
	"context"

	"github.com/nais/api/internal/audit"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/model"
)

func (r *auditEventMemberAddedResolver) Team(ctx context.Context, obj *audit.AuditEventMemberAdded) (*model.Team, error) {
	return resolveEventTeam(ctx, obj.BaseAuditEvent)
}

func (r *auditEventMemberAddedResolver) Env(ctx context.Context, obj *audit.AuditEventMemberAdded) (*model.Env, error) {
	return resolveEventEnv(ctx, obj.BaseAuditEvent)
}

func (r *auditEventMemberRemovedResolver) Team(ctx context.Context, obj *audit.AuditEventMemberRemoved) (*model.Team, error) {
	return resolveEventTeam(ctx, obj.BaseAuditEvent)
}

func (r *auditEventMemberRemovedResolver) Env(ctx context.Context, obj *audit.AuditEventMemberRemoved) (*model.Env, error) {
	return resolveEventEnv(ctx, obj.BaseAuditEvent)
}

func (r *auditEventMemberSetRoleResolver) Team(ctx context.Context, obj *audit.AuditEventMemberSetRole) (*model.Team, error) {
	return resolveEventTeam(ctx, obj.BaseAuditEvent)
}

func (r *auditEventMemberSetRoleResolver) Env(ctx context.Context, obj *audit.AuditEventMemberSetRole) (*model.Env, error) {
	return resolveEventEnv(ctx, obj.BaseAuditEvent)
}

func (r *auditEventTeamAddRepositoryResolver) Team(ctx context.Context, obj *audit.AuditEventTeamAddRepository) (*model.Team, error) {
	return resolveEventTeam(ctx, obj.BaseAuditEvent)
}

func (r *auditEventTeamAddRepositoryResolver) Env(ctx context.Context, obj *audit.AuditEventTeamAddRepository) (*model.Env, error) {
	return resolveEventEnv(ctx, obj.BaseAuditEvent)
}

func (r *auditEventTeamRemoveRepositoryResolver) Team(ctx context.Context, obj *audit.AuditEventTeamRemoveRepository) (*model.Team, error) {
	return resolveEventTeam(ctx, obj.BaseAuditEvent)
}

func (r *auditEventTeamRemoveRepositoryResolver) Env(ctx context.Context, obj *audit.AuditEventTeamRemoveRepository) (*model.Env, error) {
	return resolveEventEnv(ctx, obj.BaseAuditEvent)
}

func (r *auditEventTeamSetAlertsSlackChannelResolver) Team(ctx context.Context, obj *audit.AuditEventTeamSetAlertsSlackChannel) (*model.Team, error) {
	return resolveEventTeam(ctx, obj.BaseAuditEvent)
}

func (r *auditEventTeamSetAlertsSlackChannelResolver) Env(ctx context.Context, obj *audit.AuditEventTeamSetAlertsSlackChannel) (*model.Env, error) {
	return resolveEventEnv(ctx, obj.BaseAuditEvent)
}

func (r *auditEventTeamSetDefaultSlackChannelResolver) Team(ctx context.Context, obj *audit.AuditEventTeamSetDefaultSlackChannel) (*model.Team, error) {
	return resolveEventTeam(ctx, obj.BaseAuditEvent)
}

func (r *auditEventTeamSetDefaultSlackChannelResolver) Env(ctx context.Context, obj *audit.AuditEventTeamSetDefaultSlackChannel) (*model.Env, error) {
	return resolveEventEnv(ctx, obj.BaseAuditEvent)
}

func (r *auditEventTeamSetPurposeResolver) Team(ctx context.Context, obj *audit.AuditEventTeamSetPurpose) (*model.Team, error) {
	return resolveEventTeam(ctx, obj.BaseAuditEvent)
}

func (r *auditEventTeamSetPurposeResolver) Env(ctx context.Context, obj *audit.AuditEventTeamSetPurpose) (*model.Env, error) {
	return resolveEventEnv(ctx, obj.BaseAuditEvent)
}

func (r *baseAuditEventResolver) Team(ctx context.Context, obj *audit.BaseAuditEvent) (*model.Team, error) {
	return resolveEventTeam(ctx, *obj)
}

func (r *baseAuditEventResolver) Env(ctx context.Context, obj *audit.BaseAuditEvent) (*model.Env, error) {
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

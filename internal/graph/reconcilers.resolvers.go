package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auditlogger"
	"github.com/nais/api/internal/auditlogger/audittype"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/roles"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/pkg/protoapi"
)

func (r *mutationResolver) EnableReconciler(ctx context.Context, name string) (*model.Reconciler, error) {
	correlationID := uuid.New()

	if _, err := r.database.GetReconciler(ctx, name); err != nil {
		r.log.WithError(err).Errorf("unable to get reconciler: %q", name)
		return nil, apierror.Errorf("Unable to get reconciler: %q", name)
	}

	configs, err := r.database.GetReconcilerConfig(ctx, name, false)
	if err != nil {
		r.log.WithError(err).Errorf("unable to get reconciler config")
		return nil, apierror.Errorf("Unable to get reconciler config")
	}

	missingOptions := make([]string, 0)
	for _, config := range configs {
		if !config.Configured {
			missingOptions = append(missingOptions, string(config.Key))
		}
	}

	if len(missingOptions) != 0 {
		r.log.WithError(err).Errorf("reconciler is not fully configured")
		return nil, apierror.Errorf("Reconciler is not fully configured, missing one or more options: %s", strings.Join(missingOptions, ", "))
	}

	reconciler, err := r.database.EnableReconciler(ctx, name)
	if err != nil {
		r.log.WithError(err).Errorf("unable to enable reconciler")
		return nil, apierror.Errorf("Unable to enable reconciler")
	}

	actor := authz.ActorFromContext(ctx)
	targets := []auditlogger.Target{
		auditlogger.ReconcilerTarget(name),
	}
	fields := auditlogger.Fields{
		Action:        audittype.AuditActionGraphqlApiReconcilersEnable,
		Actor:         actor,
		CorrelationID: correlationID,
	}
	r.auditLogger.Logf(ctx, targets, fields, "Enable reconciler: %q", name)
	r.triggerEvent(ctx, protoapi.EventTypes_EVENT_RECONCILER_ENABLED, &protoapi.EventReconcilerEnabled{Reconciler: name}, correlationID)

	return toGraphReconciler(reconciler), nil
}

func (r *mutationResolver) DisableReconciler(ctx context.Context, name string) (*model.Reconciler, error) {
	var reconciler *database.Reconciler
	var err error
	err = r.database.Transaction(ctx, func(ctx context.Context, dbtx database.Database) error {
		reconciler, err = dbtx.GetReconciler(ctx, name)
		if err != nil {
			return err
		}

		reconciler, err = dbtx.DisableReconciler(ctx, name)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	correlationID := uuid.New()

	actor := authz.ActorFromContext(ctx)
	targets := []auditlogger.Target{
		auditlogger.ReconcilerTarget(name),
	}
	fields := auditlogger.Fields{
		Action:        audittype.AuditActionGraphqlApiReconcilersDisable,
		Actor:         actor,
		CorrelationID: correlationID,
	}
	r.auditLogger.Logf(ctx, targets, fields, "Disable reconciler: %q", name)
	r.triggerEvent(ctx, protoapi.EventTypes_EVENT_RECONCILER_DISABLED, &protoapi.EventReconcilerDisabled{Reconciler: name}, correlationID)

	return toGraphReconciler(reconciler), nil
}

func (r *mutationResolver) ConfigureReconciler(ctx context.Context, name string, config []*model.ReconcilerConfigInput) (*model.Reconciler, error) {
	reconcilerConfig := make(map[string]string)
	for _, entry := range config {
		reconcilerConfig[string(entry.Key)] = entry.Value
	}

	err := r.database.Transaction(ctx, func(ctx context.Context, dbtx database.Database) error {
		rows, err := dbtx.GetReconcilerConfig(ctx, name, false)
		if err != nil {
			return err
		}

		validOptions := make(map[string]struct{})
		for _, row := range rows {
			validOptions[row.Key] = struct{}{}
		}

		for key, value := range reconcilerConfig {
			if _, exists := validOptions[key]; !exists {
				keys := make([]string, 0, len(validOptions))
				for key := range validOptions {
					keys = append(keys, key)
				}
				return fmt.Errorf("unknown configuration option %q for reconciler %q. Valid options: %s", key, name, strings.Join(keys, ", "))
			}

			err = dbtx.ConfigureReconciler(ctx, name, key, value)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	reconciler, err := r.database.GetReconciler(ctx, name)
	if err != nil {
		return nil, err
	}

	correlationID := uuid.New()

	actor := authz.ActorFromContext(ctx)
	targets := []auditlogger.Target{
		auditlogger.ReconcilerTarget(name),
	}
	fields := auditlogger.Fields{
		Action:        audittype.AuditActionGraphqlApiReconcilersConfigure,
		Actor:         actor,
		CorrelationID: correlationID,
	}
	r.auditLogger.Logf(ctx, targets, fields, "Configure reconciler: %q", name)
	r.triggerEvent(ctx, protoapi.EventTypes_EVENT_RECONCILER_CONFIGURED, &protoapi.EventReconcilerConfigured{Reconciler: name}, correlationID)

	return toGraphReconciler(reconciler), nil
}

func (r *mutationResolver) ResetReconciler(ctx context.Context, name string) (*model.Reconciler, error) {
	var reconciler *database.Reconciler
	var err error
	err = r.database.Transaction(ctx, func(ctx context.Context, dbtx database.Database) error {
		reconciler, err = dbtx.ResetReconcilerConfig(ctx, name)
		if err != nil {
			return err
		}

		if !reconciler.Enabled {
			return nil
		}

		reconciler, err = dbtx.DisableReconciler(ctx, name)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	actor := authz.ActorFromContext(ctx)
	targets := []auditlogger.Target{
		auditlogger.ReconcilerTarget(name),
	}
	fields := auditlogger.Fields{
		Action: audittype.AuditActionGraphqlApiReconcilersReset,
		Actor:  actor,
	}
	r.auditLogger.Logf(ctx, targets, fields, "Reset reconciler: %q", name)

	return toGraphReconciler(reconciler), nil
}

func (r *mutationResolver) AddReconcilerOptOut(ctx context.Context, teamSlug slug.Slug, userID scalar.Ident, reconciler string) (*model.TeamMember, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamAuthorization(actor, roles.AuthorizationTeamsMembersAdmin, teamSlug)
	if err != nil {
		return nil, err
	}

	team, err := r.database.GetTeamBySlug(ctx, teamSlug)
	if err != nil {
		return nil, apierror.ErrTeamNotExist
	}

	uuid, err := userID.AsUUID()
	if err != nil {
		return nil, err
	}

	user, err := r.database.GetTeamMember(ctx, teamSlug, uuid)
	if err != nil {
		return nil, apierror.ErrUserIsNotTeamMember
	}

	err = r.database.AddReconcilerOptOut(ctx, uuid, teamSlug, reconciler)
	if err != nil {
		return nil, err
	}

	return &model.TeamMember{
		TeamSlug: team.Slug,
		UserID:   scalar.UserIdent(user.ID),
	}, nil
}

func (r *mutationResolver) RemoveReconcilerOptOut(ctx context.Context, teamSlug slug.Slug, userID scalar.Ident, reconciler string) (*model.TeamMember, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamAuthorization(actor, roles.AuthorizationTeamsMembersAdmin, teamSlug)
	if err != nil {
		return nil, err
	}

	team, err := r.database.GetTeamBySlug(ctx, teamSlug)
	if err != nil {
		return nil, apierror.ErrTeamNotExist
	}

	uuid, err := userID.AsUUID()
	if err != nil {
		return nil, err
	}
	user, err := r.database.GetTeamMember(ctx, teamSlug, uuid)
	if err != nil {
		return nil, apierror.ErrUserIsNotTeamMember
	}

	err = r.database.RemoveReconcilerOptOut(ctx, uuid, teamSlug, reconciler)
	if err != nil {
		return nil, err
	}

	return &model.TeamMember{
		TeamSlug: team.Slug,
		UserID:   scalar.UserIdent(user.ID),
	}, nil
}

func (r *queryResolver) Reconcilers(ctx context.Context, offset *int, limit *int) (*model.ReconcilerList, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireGlobalAuthorization(actor, roles.AuthorizationTeamsCreate)
	if err != nil {
		return nil, err
	}

	p := model.NewPagination(offset, limit)

	reconcilers, total, err := r.database.GetReconcilers(ctx, database.Page{
		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return nil, err
	}

	graphReconcilers := make([]*model.Reconciler, 0, len(reconcilers))
	for _, reconciler := range reconcilers {
		graphReconcilers = append(graphReconcilers, toGraphReconciler(reconciler))
	}

	return &model.ReconcilerList{
		Nodes:    graphReconcilers,
		PageInfo: model.NewPageInfo(p, total),
	}, nil
}

func (r *reconcilerResolver) Config(ctx context.Context, obj *model.Reconciler) ([]*model.ReconcilerConfig, error) {
	config, err := r.database.GetReconcilerConfig(ctx, obj.Name, false)
	if err != nil {
		return nil, err
	}

	graphConfig := make([]*model.ReconcilerConfig, 0, len(config))
	for _, entry := range config {
		graphConfig = append(graphConfig, &model.ReconcilerConfig{
			Key:         entry.Key,
			Value:       entry.Value,
			Configured:  entry.Configured,
			DisplayName: entry.DisplayName,
			Description: entry.Description,
			Secret:      entry.Secret,
		})
	}

	return graphConfig, nil
}

func (r *reconcilerResolver) Configured(ctx context.Context, obj *model.Reconciler) (bool, error) {
	configs, err := r.database.GetReconcilerConfig(ctx, obj.Name, false)
	if err != nil {
		return false, err
	}

	for _, config := range configs {
		if !config.Configured {
			return false, nil
		}
	}

	return true, nil
}

func (r *reconcilerResolver) AuditLogs(ctx context.Context, obj *model.Reconciler, offset *int, limit *int) (*model.AuditLogList, error) {
	p := model.NewPagination(offset, limit)
	dbe, total, err := r.database.GetAuditLogsForReconciler(ctx, obj.Name, database.Page{
		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return nil, err
	}

	return &model.AuditLogList{
		Nodes:    toGraphAuditLogs(dbe),
		PageInfo: model.NewPageInfo(p, total),
	}, nil
}

func (r *reconcilerResolver) Errors(ctx context.Context, obj *model.Reconciler, offset *int, limit *int) (*model.ReconcilerErrorList, error) {
	p := model.NewPagination(offset, limit)
	errors, total, err := r.database.GetReconcilerErrors(ctx, obj.Name, database.Page{
		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return nil, err
	}

	return &model.ReconcilerErrorList{
		Nodes: func([]*database.ReconcilerError) []*model.ReconcilerError {
			ret := make([]*model.ReconcilerError, len(errors))
			for i, row := range errors {
				ret[i] = &model.ReconcilerError{
					ID:            scalar.ReconcilerErrorIdent(int(row.ID)),
					CorrelationID: row.CorrelationID,
					CreatedAt:     row.CreatedAt.Time,
					Message:       row.ErrorMessage,
					TeamSlug:      row.TeamSlug,
				}
			}
			return ret
		}(errors),
		PageInfo: model.NewPageInfo(p, total),
	}, nil
}

func (r *reconcilerErrorResolver) Team(ctx context.Context, obj *model.ReconcilerError) (*model.Team, error) {
	return loader.GetTeam(ctx, obj.TeamSlug)
}

func (r *Resolver) Reconciler() gengql.ReconcilerResolver { return &reconcilerResolver{r} }

func (r *Resolver) ReconcilerError() gengql.ReconcilerErrorResolver {
	return &reconcilerErrorResolver{r}
}

type (
	reconcilerResolver      struct{ *Resolver }
	reconcilerErrorResolver struct{ *Resolver }
)

package reconciler

import (
	"context"
	"fmt"
	"strings"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/v1/databasev1"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/reconciler/reconcilersql"
)

func Get(ctx context.Context, name string) (*Reconciler, error) {
	return fromContext(ctx).reconcilerLoader.Load(ctx, name)
}

func GetByIdent(ctx context.Context, ident ident.Ident) (*Reconciler, error) {
	name, err := parseIdent(ident)
	if err != nil {
		return nil, err
	}
	return Get(ctx, name)
}

func List(ctx context.Context, page *pagination.Pagination) (*ReconcilerConnection, error) {
	q := db(ctx)

	ret, err := q.List(ctx, reconcilersql.ListParams{
		Offset: page.Offset(),
		Limit:  page.Limit(),
	})
	if err != nil {
		return nil, err
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, err
	}
	return pagination.NewConvertConnection(ret, page, int32(total), toGraphReconciler), nil
}

func GetConfig(ctx context.Context, name string, includeSecrets bool) ([]*ReconcilerConfig, error) {
	rows, err := db(ctx).GetConfig(ctx, reconcilersql.GetConfigParams{
		ReconcilerName: name,
		IncludeSecret:  includeSecrets,
	})
	if err != nil {
		return nil, err
	}

	ret := make([]*ReconcilerConfig, len(rows))
	for i, row := range rows {
		ret[i] = toGraphReconcilerConfig(name, row)
	}

	return ret, nil
}

func Enable(ctx context.Context, name string) (*Reconciler, error) {
	q := db(ctx)

	if _, err := q.Get(ctx, name); err != nil {
		return nil, apierror.Errorf("Unable to get reconciler: %q", name)
	}

	configs, err := GetConfig(ctx, name, false)
	if err != nil {
		return nil, apierror.Errorf("Unable to get reconciler config")
	}

	missingOptions := make([]string, 0)
	for _, config := range configs {
		if !config.Configured {
			missingOptions = append(missingOptions, string(config.Key))
		}
	}

	if len(missingOptions) != 0 {
		return nil, apierror.Errorf("Reconciler is not fully configured, missing one or more options: %s", strings.Join(missingOptions, ", "))
	}

	reconciler, err := q.Enable(ctx, name)
	if err != nil {
		return nil, apierror.Errorf("Unable to enable reconciler")
	}

	// TODO: Implement audit logging
	// actor := authz.ActorFromContext(ctx)
	// targets := []auditlogger.Target{
	// 	auditlogger.ReconcilerTarget(name),
	// }
	// fields := auditlogger.Fields{
	// 	Action:        audittype.AuditActionGraphqlApiReconcilersEnable,
	// 	Actor:         actor,
	// 	CorrelationID: correlationID,
	// }
	// r.auditLogger.Logf(ctx, targets, fields, "Enable reconciler: %q", name)

	return toGraphReconciler(reconciler), nil
}

func Disable(ctx context.Context, name string) (*Reconciler, error) {
	_, err := Get(ctx, name)
	if err != nil {
		return nil, err
	}

	reconcilerRow, err := db(ctx).Disable(ctx, name)
	if err != nil {
		return nil, err
	}

	// TODO: Implement audit logging
	// actor := authz.ActorFromContext(ctx)
	// targets := []auditlogger.Target{
	// 	auditlogger.ReconcilerTarget(name),
	// }
	// fields := auditlogger.Fields{
	// 	Action:        audittype.AuditActionGraphqlApiReconcilersDisable,
	// 	Actor:         actor,
	// 	CorrelationID: correlationID,
	// }
	// r.auditLogger.Logf(ctx, targets, fields, "Disable reconciler: %q", name)

	return toGraphReconciler(reconcilerRow), nil
}

func Configure(ctx context.Context, name string, config []*ReconcilerConfigInput) (*Reconciler, error) {
	reconcilerConfig := make(map[string]string)
	for _, entry := range config {
		reconcilerConfig[string(entry.Key)] = entry.Value
	}

	err := databasev1.Transaction(ctx, func(ctx context.Context) error {
		rows, err := GetConfig(ctx, name, false)
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

			err = db(ctx).Configure(ctx, reconcilersql.ConfigureParams{
				ReconcilerName: name,
				Key:            key,
				Value:          value,
			})
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	reconciler, err := Get(ctx, name)
	if err != nil {
		return nil, err
	}

	// TODO: Implement audit logging
	// correlationID := uuid.New()
	// actor := authz.ActorFromContext(ctx)
	// targets := []auditlogger.Target{
	// 	auditlogger.ReconcilerTarget(name),
	// }
	// fields := auditlogger.Fields{
	// 	Action:        audittype.AuditActionGraphqlApiReconcilersConfigure,
	// 	Actor:         actor,
	// 	CorrelationID: correlationID,
	// }
	// r.auditLogger.Logf(ctx, targets, fields, "Configure reconciler: %q", name)

	return reconciler, nil
}

func GetErrors(ctx context.Context, reconcilerName string, page *pagination.Pagination) (*ReconcilerErrorConnection, error) {
	q := db(ctx)

	ret, err := q.GetErrors(ctx, reconcilersql.GetErrorsParams{
		Reconciler: reconcilerName,
		Offset:     page.Offset(),
		Limit:      page.Limit(),
	})
	if err != nil {
		return nil, err
	}

	total, err := q.GetErrorsCount(ctx, reconcilerName)
	if err != nil {
		return nil, err
	}

	return pagination.NewConvertConnection(ret, page, int32(total), toGraphReconcilerError), nil
}

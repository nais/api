package serviceaccount

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/serviceaccount/serviceaccountsql"
	"github.com/nais/api/internal/slug"
)

// throttleLastUsedAt determines how often the last_used_at column may be updated. We avoid a write per
// authenticated request by only updating when the existing value is older than this threshold.
const throttleLastUsedAt = time.Minute

func GetBinding(ctx context.Context, id uuid.UUID) (*ServiceAccountWorkloadBinding, error) {
	b, err := fromContext(ctx).serviceAccountBindingLoader.Load(ctx, id)
	if err != nil {
		return nil, handleBindingError(err)
	}
	return b, nil
}

func GetBindingByIdent(ctx context.Context, id ident.Ident) (*ServiceAccountWorkloadBinding, error) {
	uid, err := parseBindingIdent(id)
	if err != nil {
		return nil, err
	}
	return GetBinding(ctx, uid)
}

// GetBindingForWorkload returns the binding pointing at the given workload, if any. Returns ErrBindingNotFound if
// no binding exists.
func GetBindingForWorkload(ctx context.Context, environment string, teamSlug slug.Slug, workloadName string) (*ServiceAccountWorkloadBinding, error) {
	b, err := db(ctx).GetBindingByWorkload(ctx, serviceaccountsql.GetBindingByWorkloadParams{
		Environment:  environment,
		TeamSlug:     teamSlug,
		WorkloadName: workloadName,
	})
	if err != nil {
		return nil, handleBindingError(err)
	}
	return toGraphServiceAccountWorkloadBinding(b), nil
}

// AddWorkloadBinding creates a new binding between a workload and a service account. The actor must be a member of the
// team that owns the service account. Returns an error if the workload is already bound.
func AddWorkloadBinding(ctx context.Context, input AddWorkloadToServiceAccountInput) (*ServiceAccount, *ServiceAccountWorkloadBinding, error) {
	sa, err := GetByIdent(ctx, input.ServiceAccountID)
	if err != nil {
		return nil, nil, err
	}

	if err := authz.CanUpdateServiceAccounts(ctx, sa.TeamSlug); err != nil {
		return nil, nil, err
	}

	if input.Environment == "" {
		return nil, nil, apierror.Errorf("Environment must not be empty.")
	}
	if input.WorkloadName == "" {
		return nil, nil, apierror.Errorf("Workload name must not be empty.")
	}

	if sa.TeamSlug != nil && *sa.TeamSlug != input.TeamSlug {
		return nil, nil, apierror.Errorf("The service account %q belongs to team %q and cannot be bound to a workload in team %q.", sa.Name, *sa.TeamSlug, input.TeamSlug)
	}

	// Check for an existing binding for this workload.
	if existing, err := GetBindingForWorkload(ctx, input.Environment, input.TeamSlug, input.WorkloadName); err == nil && existing != nil {
		return nil, nil, apierror.Errorf("The workload %q in team %q is already bound to a service account.", input.WorkloadName, input.TeamSlug)
	}

	var binding *serviceaccountsql.ServiceAccountWorkloadBinding
	err = database.Transaction(ctx, func(ctx context.Context) error {
		var err error
		binding, err = db(ctx).CreateBinding(ctx, serviceaccountsql.CreateBindingParams{
			ServiceAccountID: sa.UUID,
			Environment:      input.Environment,
			TeamSlug:         input.TeamSlug,
			WorkloadName:     input.WorkloadName,
		})
		if err != nil {
			return err
		}

		return activitylog.Create(ctx, activitylog.CreateInput{
			Action:          activityLogEntryActionAddServiceAccountWorkloadBinding,
			Actor:           authz.ActorFromContext(ctx).User,
			ResourceType:    activityLogEntryResourceTypeServiceAccount,
			ResourceName:    sa.Name,
			TeamSlug:        sa.TeamSlug,
			EnvironmentName: &input.Environment,
			Data: &ServiceAccountWorkloadBindingAddedActivityLogEntryData{
				TeamSlug:     input.TeamSlug.String(),
				WorkloadName: input.WorkloadName,
			},
		})
	})
	if err != nil {
		return nil, nil, err
	}

	return sa, toGraphServiceAccountWorkloadBinding(binding), nil
}

// RemoveWorkloadBinding removes a binding by ID. The actor must be a member of the team that owns the service
// account.
func RemoveWorkloadBinding(ctx context.Context, input RemoveWorkloadFromServiceAccountInput) (*ServiceAccount, error) {
	binding, err := GetBindingByIdent(ctx, input.BindingID)
	if err != nil {
		return nil, err
	}

	sa, err := Get(ctx, binding.ServiceAccountID)
	if err != nil {
		return nil, err
	}

	if err := authz.CanUpdateServiceAccounts(ctx, sa.TeamSlug); err != nil {
		return nil, err
	}

	err = database.Transaction(ctx, func(ctx context.Context) error {
		if err := db(ctx).DeleteBinding(ctx, binding.UUID); err != nil {
			return err
		}

		env := binding.Environment
		return activitylog.Create(ctx, activitylog.CreateInput{
			Action:          activityLogEntryActionRemoveServiceAccountWorkloadBinding,
			Actor:           authz.ActorFromContext(ctx).User,
			ResourceType:    activityLogEntryResourceTypeServiceAccount,
			ResourceName:    sa.Name,
			TeamSlug:        sa.TeamSlug,
			EnvironmentName: &env,
			Data: &ServiceAccountWorkloadBindingRemovedActivityLogEntryData{
				TeamSlug:     binding.TeamSlug.String(),
				WorkloadName: binding.WorkloadName,
			},
		})
	})
	if err != nil {
		return nil, err
	}

	return sa, nil
}

func ListBindingsForServiceAccount(ctx context.Context, page *pagination.Pagination, serviceAccountID uuid.UUID) (*ServiceAccountWorkloadBindingConnection, error) {
	ret, err := db(ctx).ListBindingsForServiceAccount(ctx, serviceaccountsql.ListBindingsForServiceAccountParams{
		ServiceAccountID: serviceAccountID,
		Offset:           page.Offset(),
		Limit:            page.Limit(),
	})
	if err != nil {
		return nil, err
	}

	var total int64
	if len(ret) > 0 {
		total = ret[0].TotalCount
	}
	return pagination.NewConvertConnection(ret, page, total, func(from *serviceaccountsql.ListBindingsForServiceAccountRow) *ServiceAccountWorkloadBinding {
		return toGraphServiceAccountWorkloadBinding(&from.ServiceAccountWorkloadBinding)
	}), nil
}

// AuthLookupResult contains the result of a successful K8s SA token authentication lookup.
type AuthLookupResult struct {
	ServiceAccount *ServiceAccount
	Binding        *ServiceAccountWorkloadBinding
}

// AuthenticateKubernetesServiceAccount looks up a binding by (environment, namespace, k8s sa name) and validates the
// kubernetes service account UID using trust-on-first-use semantics. On a successful match, last_used_at is updated
// (subject to throttling).
func AuthenticateKubernetesServiceAccount(ctx context.Context, environment string, teamSlug slug.Slug, k8sServiceAccountName string, k8sServiceAccountUID uuid.UUID) (*AuthLookupResult, error) {
	q := db(ctx)
	row, err := q.GetBindingByWorkload(ctx, serviceaccountsql.GetBindingByWorkloadParams{
		Environment:  environment,
		TeamSlug:     teamSlug,
		WorkloadName: k8sServiceAccountName,
	})
	if err != nil {
		return nil, handleBindingError(err)
	}

	if row.KubernetesServiceAccountUid == nil {
		// Trust on first use: pin the UID.
		err := q.SetBindingKubernetesUID(ctx, serviceaccountsql.SetBindingKubernetesUIDParams{
			KubernetesServiceAccountUid: &k8sServiceAccountUID,
			ID:                          row.ID,
		})
		if err != nil {
			return nil, err
		}
		row.KubernetesServiceAccountUid = &k8sServiceAccountUID
	} else if *row.KubernetesServiceAccountUid != k8sServiceAccountUID {
		return nil, ErrUIDMismatch
	}

	sa, err := Get(ctx, row.ServiceAccountID)
	if err != nil {
		return nil, err
	}

	// Throttled last_used_at update.
	now := time.Now()
	if !row.LastUsedAt.Valid || now.Sub(row.LastUsedAt.Time) > throttleLastUsedAt {
		_ = q.UpdateBindingLastUsedAt(ctx, row.ID)
	}

	return &AuthLookupResult{
		ServiceAccount: sa,
		Binding:        toGraphServiceAccountWorkloadBinding(row),
	}, nil
}

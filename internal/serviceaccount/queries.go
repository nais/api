package serviceaccount

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/serviceaccount/serviceaccountsql"
)

func Get(ctx context.Context, serviceAccountID uuid.UUID) (*ServiceAccount, error) {
	sa, err := fromContext(ctx).serviceAccountLoader.Load(ctx, serviceAccountID)
	if err != nil {
		return nil, handleError(err)
	}
	return sa, nil
}

func GetByToken(ctx context.Context, token string) (*ServiceAccount, error) {
	sa, err := db(ctx).GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	return toGraphServiceAccount(sa), nil
}

func GetByName(ctx context.Context, name string) (*ServiceAccount, error) {
	sa, err := db(ctx).GetByName(ctx, name)
	if err != nil {
		return nil, err
	}

	return toGraphServiceAccount(sa), nil
}

func GetByIdent(ctx context.Context, ident ident.Ident) (*ServiceAccount, error) {
	uid, err := parseIdent(ident)
	if err != nil {
		return nil, err
	}
	return Get(ctx, uid)
}

func Create(ctx context.Context, input CreateServiceAccountInput) (*ServiceAccount, error) {
	if err := authz.CanCreateServiceAccounts(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	var sa *ServiceAccount
	err := database.Transaction(ctx, func(ctx context.Context) error {
		dbSA, err := db(ctx).Create(ctx, serviceaccountsql.CreateParams{
			Name:        input.Name,
			Description: input.Description,
			TeamSlug:    input.TeamSlug,
		})
		if err != nil {
			return err
		}

		sa = toGraphServiceAccount(dbSA)

		actor := authz.ActorFromContext(ctx)

		return activitylog.Create(ctx, activitylog.CreateInput{
			Action:       activitylog.ActivityLogEntryActionCreated,
			Actor:        actor.User,
			ResourceType: activityLogEntryResourceTypeServiceAccount,
			ResourceName: dbSA.Name,
			TeamSlug:     dbSA.TeamSlug,
		})
	})
	if err != nil {
		return nil, err
	}

	return sa, nil
}

func Update(ctx context.Context, input UpdateServiceAccountInput) (*ServiceAccount, error) {
	existingSA, err := GetByIdent(ctx, input.ID)
	if err != nil {
		return nil, err
	}

	if err := authz.CanUpdateServiceAccounts(ctx, existingSA.TeamSlug); err != nil {
		return nil, err
	}

	var sa *ServiceAccount
	err = database.Transaction(ctx, func(ctx context.Context) error {
		dbSA, err := db(ctx).Update(ctx, serviceaccountsql.UpdateParams{
			ID:          existingSA.UUID,
			Description: input.Description,
		})
		if err != nil {
			return err
		}

		sa = toGraphServiceAccount(dbSA)

		updatedFields := make([]*ServiceAccountUpdatedActivityLogEntryDataUpdatedField, 0)
		if input.Description != nil && *input.Description != existingSA.Description {
			updatedFields = append(updatedFields, &ServiceAccountUpdatedActivityLogEntryDataUpdatedField{
				Field:    "description",
				OldValue: &existingSA.Description,
				NewValue: input.Description,
			})
		}

		return activitylog.Create(ctx, activitylog.CreateInput{
			Action:       activitylog.ActivityLogEntryActionUpdated,
			Actor:        authz.ActorFromContext(ctx).User,
			ResourceType: activityLogEntryResourceTypeServiceAccount,
			ResourceName: sa.Name,
			TeamSlug:     sa.TeamSlug,
			Data: func(fields []*ServiceAccountUpdatedActivityLogEntryDataUpdatedField) *ServiceAccountUpdatedActivityLogEntryData {
				if len(fields) == 0 {
					return nil
				}

				return &ServiceAccountUpdatedActivityLogEntryData{
					UpdatedFields: fields,
				}
			}(updatedFields),
		})
	})
	if err != nil {
		return nil, err
	}

	return sa, nil
}

func Delete(ctx context.Context, input DeleteServiceAccountInput) error {
	existingSA, err := GetByIdent(ctx, input.ID)
	if err != nil {
		return err
	}

	if err := authz.CanDeleteServiceAccounts(ctx, existingSA.TeamSlug); err != nil {
		return err
	}

	return database.Transaction(ctx, func(ctx context.Context) error {
		if err := db(ctx).Delete(ctx, existingSA.UUID); err != nil {
			return err
		}

		return activitylog.Create(ctx, activitylog.CreateInput{
			Action:       activitylog.ActivityLogEntryActionDeleted,
			Actor:        authz.ActorFromContext(ctx).User,
			ResourceType: activityLogEntryResourceTypeServiceAccount,
			ResourceName: existingSA.Name,
			TeamSlug:     existingSA.TeamSlug,
		})
	})
}

func RemoveApiKeysFromServiceAccount(ctx context.Context, serviceAccountID uuid.UUID) error {
	return db(ctx).RemoveApiKeysFromServiceAccount(ctx, serviceAccountID)
}

func CreateToken(ctx context.Context, token string, serviceAccountID uuid.UUID) error {
	return db(ctx).CreateToken(ctx, serviceaccountsql.CreateTokenParams{
		// ExpiresAt: ...,
		Note:             "some note",
		Token:            token,
		ServiceAccountID: serviceAccountID,
	})
}

func List(ctx context.Context, page *pagination.Pagination) (*ServiceAccountConnection, error) {
	q := db(ctx)

	ret, err := q.List(ctx, serviceaccountsql.ListParams{
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
	return pagination.NewConvertConnection(ret, page, total, toGraphServiceAccount), nil
}

func DeleteStatic(ctx context.Context, id uuid.UUID) error {
	return db(ctx).Delete(ctx, id)
}

func AssignRole(ctx context.Context, input AssignRoleToServiceAccountInput) (*ServiceAccount, error) {
	sa, err := GetByIdent(ctx, input.ServiceAccountID)
	if err != nil {
		return nil, err
	}

	// Check if user has permission to update service account
	if err := authz.CanUpdateServiceAccounts(ctx, sa.TeamSlug); err != nil {
		return nil, err
	}

	role, err := authz.GetRole(ctx, input.RoleName)
	if err != nil {
		return nil, err
	}

	if hasRole, err := authz.ServiceAccountHasRole(ctx, sa.UUID, role.Name); err != nil {
		return nil, err
	} else if hasRole {
		return nil, apierror.Errorf("Service account already has already been assigned the %q role.", role.Name)
	}

	if role.OnlyGlobal && sa.TeamSlug != nil {
		return nil, apierror.Errorf("Role %q is only allowed on global service accounts.", input.RoleName)
	}

	if err := authz.AssignRoleToServiceAccount(ctx, sa.UUID, role.Name); err != nil {
		return nil, err
	}

	return sa, nil
}

func RevokeRole(ctx context.Context, input RevokeRoleFromServiceAccountInput) (*ServiceAccount, error) {
	sa, err := GetByIdent(ctx, input.ServiceAccountID)
	if err != nil {
		return nil, err
	}

	if err := authz.CanUpdateServiceAccounts(ctx, sa.TeamSlug); err != nil {
		return nil, err
	}

	role, err := authz.GetRole(ctx, input.RoleName)
	if err != nil {
		return nil, err
	}

	if hasRole, err := authz.ServiceAccountHasRole(ctx, sa.UUID, role.Name); err != nil {
		return nil, err
	} else if !hasRole {
		return nil, apierror.Errorf("Service account does not have the %q role assigned.", role.Name)
	}

	if err := authz.RevokeRoleFromServiceAccount(ctx, sa.UUID, role.Name); err != nil {
		return nil, err
	}

	return sa, nil
}

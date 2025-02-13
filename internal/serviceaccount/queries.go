package serviceaccount

import (
	"context"
	"crypto/rand"

	"github.com/btcsuite/btcutil/base58"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/serviceaccount/serviceaccountsql"
	"k8s.io/utils/ptr"
)

func Get(ctx context.Context, serviceAccountID uuid.UUID) (*ServiceAccount, error) {
	sa, err := fromContext(ctx).serviceAccountLoader.Load(ctx, serviceAccountID)
	if err != nil {
		return nil, handleError(err)
	}
	return sa, nil
}

func GetToken(ctx context.Context, serviceAccountTokenID uuid.UUID) (*ServiceAccountToken, error) {
	sa, err := fromContext(ctx).serviceAccountTokenLoader.Load(ctx, serviceAccountTokenID)
	if err != nil {
		return nil, err
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

func GetByIdent(ctx context.Context, ident ident.Ident) (*ServiceAccount, error) {
	uid, err := parseIdent(ident)
	if err != nil {
		return nil, err
	}
	return Get(ctx, uid)
}

func GetTokenByIdent(ctx context.Context, ident ident.Ident) (*ServiceAccountToken, error) {
	uid, err := parseTokenIdent(ident)
	if err != nil {
		return nil, err
	}
	return GetToken(ctx, uid)
}

func Create(ctx context.Context, input CreateServiceAccountInput) (*ServiceAccount, error) {
	if err := authz.CanCreateServiceAccounts(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	var sa *serviceaccountsql.ServiceAccount
	err := database.Transaction(ctx, func(ctx context.Context) error {
		var err error
		sa, err = db(ctx).Create(ctx, serviceaccountsql.CreateParams{
			Name:        input.Name,
			Description: input.Description,
			TeamSlug:    input.TeamSlug,
		})
		if err != nil {
			return err
		}

		return activitylog.Create(ctx, activitylog.CreateInput{
			Action:       activitylog.ActivityLogEntryActionCreated,
			Actor:        authz.ActorFromContext(ctx).User,
			ResourceType: activityLogEntryResourceTypeServiceAccount,
			ResourceName: sa.Name,
			TeamSlug:     sa.TeamSlug,
		})
	})
	if err != nil {
		return nil, err
	}

	return toGraphServiceAccount(sa), nil
}

func Update(ctx context.Context, input UpdateServiceAccountInput) (*ServiceAccount, error) {
	existingSA, err := GetByIdent(ctx, input.ServiceAccountID)
	if err != nil {
		return nil, err
	}

	if err := authz.CanUpdateServiceAccounts(ctx, existingSA.TeamSlug); err != nil {
		return nil, err
	}

	var sa *serviceaccountsql.ServiceAccount
	err = database.Transaction(ctx, func(ctx context.Context) error {
		var err error
		sa, err = db(ctx).Update(ctx, serviceaccountsql.UpdateParams{
			ID:          existingSA.UUID,
			Description: input.Description,
		})
		if err != nil {
			return err
		}

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

	return toGraphServiceAccount(sa), nil
}

func Delete(ctx context.Context, input DeleteServiceAccountInput) error {
	existingSA, err := GetByIdent(ctx, input.ServiceAccountID)
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

func CreateToken(ctx context.Context, input CreateServiceAccountTokenInput) (*ServiceAccount, *ServiceAccountToken, *string, error) {
	sa, err := GetByIdent(ctx, input.ServiceAccountID)
	if err != nil {
		return nil, nil, nil, err
	}

	if err := authz.CanUpdateServiceAccounts(ctx, sa.TeamSlug); err != nil {
		return nil, nil, nil, err
	}

	secret, err := getToken()
	if err != nil {
		return nil, nil, nil, err
	}

	expiresAt := pgtype.Date{}
	if input.ExpiresAt != nil && !input.ExpiresAt.Time().IsZero() {
		expiresAt.Time = input.ExpiresAt.Time()
		expiresAt.Valid = true
	}

	var t *serviceaccountsql.ServiceAccountToken
	err = database.Transaction(ctx, func(ctx context.Context) error {
		t, err = db(ctx).CreateToken(ctx, serviceaccountsql.CreateTokenParams{
			ExpiresAt:        expiresAt,
			Description:      input.Description,
			Token:            *secret,
			ServiceAccountID: sa.UUID,
		})
		if err != nil {
			return err
		}

		return nil

		// TODO
		return activitylog.Create(ctx, activitylog.CreateInput{
			// ...
		})
	})
	if err != nil {
		return nil, nil, nil, err
	}

	return sa, toGraphServiceAccountToken(t), secret, nil
}

func UpdateToken(ctx context.Context, input UpdateServiceAccountTokenInput) (*ServiceAccount, *ServiceAccountToken, error) {
	token, err := GetTokenByIdent(ctx, input.ServiceAccountTokenID)
	if err != nil {
		return nil, nil, err
	}

	sa, err := Get(ctx, token.ServiceAccountID)
	if err != nil {
		return nil, nil, err
	}

	if err := authz.CanUpdateServiceAccounts(ctx, sa.TeamSlug); err != nil {
		return nil, nil, err
	}

	expiresAt := token.ExpiresAt.PgDate()
	if e := input.ExpiresAt; e != nil {
		if e.ExpiresAt == nil && e.RemoveExpiry == nil {
			return nil, nil, apierror.Errorf("Either expiresAt or removeExpiry must be set.")
		} else if e.ExpiresAt != nil {
			expiresAt = e.ExpiresAt.PgDate()
		} else if *e.RemoveExpiry {
			expiresAt = pgtype.Date{}
		}
	}

	var t *serviceaccountsql.ServiceAccountToken
	err = database.Transaction(ctx, func(ctx context.Context) error {
		t, err = db(ctx).UpdateToken(ctx, serviceaccountsql.UpdateTokenParams{
			ID:          token.UUID,
			ExpiresAt:   expiresAt,
			Description: input.Description,
		})
		if err != nil {
			return err
		}

		return nil

		// TODO
		return activitylog.Create(ctx, activitylog.CreateInput{
			// ...
		})
	})
	if err != nil {
		return nil, nil, err
	}

	return sa, toGraphServiceAccountToken(t), nil
}

func DeleteToken(ctx context.Context, input DeleteServiceAccountTokenInput) (*ServiceAccount, error) {
	token, err := GetTokenByIdent(ctx, input.ServiceAccountTokenID)
	if err != nil {
		return nil, err
	}

	sa, err := Get(ctx, token.ServiceAccountID)
	if err != nil {
		return nil, err
	}

	if err := authz.CanUpdateServiceAccounts(ctx, sa.TeamSlug); err != nil {
		return nil, err
	}

	err = database.Transaction(ctx, func(ctx context.Context) error {
		if err := db(ctx).DeleteToken(ctx, token.UUID); err != nil {
			return err
		}

		return nil

		// TODO
		return activitylog.Create(ctx, activitylog.CreateInput{
			// ...
		})
	})
	if err != nil {
		return nil, err
	}

	return sa, nil
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

func ListTokensForServiceAccount(ctx context.Context, page *pagination.Pagination, serviceAccountID uuid.UUID) (*ServiceAccountTokenConnection, error) {
	q := db(ctx)

	ret, err := q.ListTokensForServiceAccount(ctx, serviceaccountsql.ListTokensForServiceAccountParams{
		ServiceAccountID: serviceAccountID,
		Offset:           page.Offset(),
		Limit:            page.Limit(),
	})
	if err != nil {
		return nil, err
	}

	total, err := q.CountTokensForServiceAccount(ctx, serviceAccountID)
	if err != nil {
		return nil, err
	}
	return pagination.NewConvertConnection(ret, page, total, toGraphServiceAccountToken), nil
}

// TODO: Remove once static service accounts has been removed
func DeleteStaticServiceAccounts(ctx context.Context) error {
	return db(ctx).DeleteStaticServiceAccounts(ctx)
}

// TODO: Remove once static service accounts has been removed
func CreateStaticServiceAccount(ctx context.Context, name string, roles []string, secret string) error {
	sa, err := db(ctx).Create(ctx, serviceaccountsql.CreateParams{
		Name:        name,
		Description: "Static service account created by Nais",
	})
	if err != nil {
		return err
	}

	for _, r := range roles {
		if err := authz.AssignRoleToServiceAccount(ctx, sa.ID, r); err != nil {
			return err
		}
	}

	_, err = db(ctx).CreateToken(ctx, serviceaccountsql.CreateTokenParams{
		Description:      "Token created by Nais",
		Token:            secret,
		ServiceAccountID: sa.ID,
	})
	if err != nil {
		return err
	}

	return nil
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

	if ok, err := authz.CanAssignRole(ctx, input.RoleName, sa.TeamSlug); err != nil {
		return nil, err
	} else if !ok {
		return nil, apierror.Errorf("User does not have permission to assign the %q role.", role.Name)
	}

	if hasRole, err := authz.ServiceAccountHasRole(ctx, sa.UUID, role.Name); err != nil {
		return nil, err
	} else if hasRole {
		return nil, apierror.Errorf("Service account already has already been assigned the %q role.", role.Name)
	}

	if role.OnlyGlobal && sa.TeamSlug != nil {
		return nil, apierror.Errorf("Role %q is only allowed on global service accounts.", input.RoleName)
	}

	err = database.Transaction(ctx, func(ctx context.Context) error {
		if err := authz.AssignRoleToServiceAccount(ctx, sa.UUID, role.Name); err != nil {
			return err
		}

		return nil

		// TODO
		return activitylog.Create(ctx, activitylog.CreateInput{
			// ...
		})
	})
	if err != nil {
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

	if ok, err := authz.CanAssignRole(ctx, input.RoleName, sa.TeamSlug); err != nil {
		return nil, err
	} else if !ok {
		return nil, apierror.Errorf("User does not have permission to revoke the %q role.", role.Name)
	}

	if hasRole, err := authz.ServiceAccountHasRole(ctx, sa.UUID, role.Name); err != nil {
		return nil, err
	} else if !hasRole {
		return nil, apierror.Errorf("Service account does not have the %q role assigned.", role.Name)
	}

	err = database.Transaction(ctx, func(ctx context.Context) error {
		if err := authz.RevokeRoleFromServiceAccount(ctx, sa.UUID, role.Name); err != nil {
			return err
		}

		return nil

		// TODO
		return activitylog.Create(ctx, activitylog.CreateInput{
			// ...
		})
	})
	if err != nil {
		return nil, err
	}

	return sa, nil
}

func getToken() (*string, error) {
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}

	return ptr.To("nais_console_" + base58.Encode(b)), nil
}

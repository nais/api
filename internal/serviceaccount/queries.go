package serviceaccount

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/role"
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

	actor := authz.ActorFromContext(ctx)
	if existingSA.TeamSlug == nil {
		err := authz.RequireGlobalAuthorization(actor, role.AuthorizationServiceAccountsUpdate)
		if err != nil {
			return nil, err
		}
	} else {
		err := authz.RequireTeamAuthorization(actor, role.AuthorizationServiceAccountsUpdate, *existingSA.TeamSlug)
		if err != nil {
			return nil, err
		}
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
			Actor:        actor.User,
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

func Delete(ctx context.Context, id uuid.UUID) error {
	return db(ctx).Delete(ctx, id)
}

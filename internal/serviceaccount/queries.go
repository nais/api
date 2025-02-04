package serviceaccount

import (
	"context"

	"github.com/google/uuid"
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
	sa, err := db(ctx).Create(ctx, serviceaccountsql.CreateParams{
		Name:        input.Name,
		Description: input.Description,
		TeamSlug:    input.TeamSlug,
	})
	if err != nil {
		return nil, err
	}

	return toGraphServiceAccount(sa), nil
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

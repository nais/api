package user

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/user/usersql"
)

func Get(ctx context.Context, userID uuid.UUID) (*User, error) {
	user, err := fromContext(ctx).userLoader.Load(ctx, userID)
	if err != nil {
		return nil, handleError(err)
	}
	return user, nil
}

func GetByIdent(ctx context.Context, ident ident.Ident) (*User, error) {
	uid, err := parseIdent(ident)
	if err != nil {
		return nil, err
	}
	return Get(ctx, uid)
}

func List(ctx context.Context, page *pagination.Pagination, orderBy *UserOrder) (*UserConnection, error) {
	q := db(ctx)

	ret, err := q.List(ctx, usersql.ListParams{
		Offset:  page.Offset(),
		Limit:   page.Limit(),
		OrderBy: orderBy.String(),
	})
	if err != nil {
		return nil, err
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, err
	}
	return pagination.NewConvertConnection(ret, page, int32(total), toGraphUser), nil
}

func GetByEmail(ctx context.Context, email string) (*User, error) {
	u, err := db(ctx).GetByEmail(ctx, email)
	if err != nil {
		return nil, handleError(err)
	}
	return toGraphUser(u), nil
}
package users

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graphv1/pagination"
	users "github.com/nais/api/internal/users/gensql"
)

func Get(ctx context.Context, userID uuid.UUID) (*User, error) {
	return fromContext(ctx).userLoader.Load(ctx, userID)
}

func GetByEmail(ctx context.Context, email string) (*User, error) {
	user, err := fromContext(ctx).db.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	return toGraphUser(user), nil
}

func List(ctx context.Context, page *pagination.Pagination, orderBy *UserOrder) (*UserConnection, error) {
	db := fromContext(ctx).db

	ret, err := db.List(ctx, users.ListParams{
		Offset:  page.Offset(),
		Limit:   page.Limit(),
		OrderBy: orderBy.String(),
	})
	if err != nil {
		return nil, err
	}

	total, err := db.Count(ctx)
	if err != nil {
		return nil, err
	}
	return pagination.NewConvertConnection(ret, page, int32(total), toGraphUser), nil
}

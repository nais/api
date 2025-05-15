package user

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/user/usersql"
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

	total := 0
	if len(ret) > 0 {
		total = int(ret[0].TotalCount)
	}
	return pagination.NewConvertConnection(ret, page, total, func(from *usersql.ListRow) *User {
		return toGraphUser(&from.User)
	}), nil
}

func GetByEmail(ctx context.Context, email string) (*User, error) {
	u, err := db(ctx).GetByEmail(ctx, email)
	if err != nil {
		return nil, handleError(err)
	}
	return toGraphUser(u), nil
}

func ListGCPGroupsForUser(ctx context.Context, userID uuid.UUID) ([]string, error) {
	return db(ctx).ListGCPGroupsForUser(ctx, userID)
}

func GetByExternalID(ctx context.Context, externalID string) (*User, error) {
	u, err := db(ctx).GetByExternalID(ctx, externalID)
	if err != nil {
		return nil, handleError(err)
	}
	return toGraphUser(u), nil
}

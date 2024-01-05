package dataloader

import (
	"context"

	"github.com/google/uuid"
	"github.com/graph-gophers/dataloader/v7"
	"github.com/nais/api/internal/db"
	"github.com/nais/api/internal/metrics"
)

type UserRoleReader struct {
	db db.Database
}

const LoaderNameUserRoles = "user_roles"

func (r *UserRoleReader) load(ctx context.Context, keys []string) []*dataloader.Result[[]*db.UserRole] {
	userRoles, err := r.db.GetAllUserRoles(ctx)
	if err != nil {
		panic(err)
	}

	userRolesByUserID := map[string][]*db.UserRole{}
	for _, u := range userRoles {
		current := userRolesByUserID[u.UserID.String()]
		userRolesByUserID[u.UserID.String()] = append(current, u)
	}

	output := make([]*dataloader.Result[[]*db.UserRole], len(keys))
	for index, userKey := range keys {
		roles := userRolesByUserID[userKey]
		output[index] = &dataloader.Result[[]*db.UserRole]{Data: roles, Error: nil}
	}

	metrics.IncDataloaderLoads(LoaderNameUserRoles)
	return output
}

func (r *UserRoleReader) newCache() dataloader.Cache[string, []*db.UserRole] {
	return dataloader.NewCache[string, []*db.UserRole]()
}

func GetUserRoles(ctx context.Context, userID uuid.UUID) ([]*db.UserRole, error) {
	metrics.IncDataloaderCalls(LoaderNameUserRoles)
	loaders := For(ctx)
	thunk := loaders.UserRolesLoader.Load(ctx, userID.String())
	result, err := thunk()
	if err != nil {
		return nil, err
	}
	return result, nil
}

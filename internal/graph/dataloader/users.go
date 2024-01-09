package dataloader

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/graph-gophers/dataloader/v7"
	db "github.com/nais/api/internal/database"
	"github.com/nais/api/internal/metrics"
)

type UserReader struct {
	db db.Database
}

const LoaderNameUsers = "users"

func (r *UserReader) load(ctx context.Context, keys []string) []*dataloader.Result[*db.User] {
	// TODO (only fetch users requested by keys var)
	users, _, err := r.db.GetUsers(ctx, 0, 10000)
	if err != nil {
		panic(err)
	}

	userById := map[string]*db.User{}
	for _, u := range users {
		userById[u.ID.String()] = u
	}

	output := make([]*dataloader.Result[*db.User], len(keys))
	for index, key := range keys {
		user, ok := userById[key]
		if ok {
			output[index] = &dataloader.Result[*db.User]{Data: user, Error: nil}
		} else {
			err := fmt.Errorf("user not found %s", key)
			output[index] = &dataloader.Result[*db.User]{Data: nil, Error: err}
		}
	}

	metrics.IncDataloaderLoads(LoaderNameUsers)
	return output
}

func (r *UserReader) newCache() dataloader.Cache[string, *db.User] {
	return dataloader.NewCache[string, *db.User]()
}

func GetUser(ctx context.Context, userID *uuid.UUID) (*db.User, error) {
	metrics.IncDataloaderCalls(LoaderNameUsers)
	loaders := For(ctx)
	thunk := loaders.UsersLoader.Load(ctx, userID.String())
	result, err := thunk()
	if err != nil {
		return nil, err
	}
	return result, nil
}

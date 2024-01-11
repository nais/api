package dataloader

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/graph-gophers/dataloader/v7"
	db "github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/metrics"
)

func ToGraphUser(m *db.User) *model.User {
	return &model.User{
		ID:         scalar.UserIdent(m.ID),
		Email:      m.Email,
		Name:       m.Name,
		ExternalID: m.ExternalID,
	}
}

type UserReader struct {
	db db.Database
}

const LoaderNameUsers = "users"

func (r *UserReader) load(ctx context.Context, keys []string) []*dataloader.Result[*model.User] {
	// TODO (only fetch users requested by keys var)
	users, _, err := r.db.GetUsers(ctx, 0, 10000)
	if err != nil {
		panic(err)
	}

	userById := map[string]*model.User{}
	for _, u := range users {
		userById[u.ID.String()] = ToGraphUser(u)
	}

	output := make([]*dataloader.Result[*model.User], len(keys))
	for index, key := range keys {
		user, ok := userById[key]
		if ok {
			output[index] = &dataloader.Result[*model.User]{Data: user, Error: nil}
		} else {
			err := fmt.Errorf("user not found %s", key)
			output[index] = &dataloader.Result[*model.User]{Data: nil, Error: err}
		}
	}

	metrics.IncDataloaderLoads(LoaderNameUsers)
	return output
}

func (r *UserReader) newCache() dataloader.Cache[string, *model.User] {
	return dataloader.NewCache[string, *model.User]()
}

func GetUser(ctx context.Context, userID *uuid.UUID) (*model.User, error) {
	metrics.IncDataloaderCalls(LoaderNameUsers)
	loaders := For(ctx)
	thunk := loaders.UsersLoader.Load(ctx, userID.String())
	result, err := thunk()
	if err != nil {
		return nil, err
	}
	return result, nil
}

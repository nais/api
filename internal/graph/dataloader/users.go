package dataloader

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/graph-gophers/dataloader/v7"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/metrics"
)

func ToGraphUser(m *database.User) *model.User {
	return &model.User{
		ID:         scalar.UserIdent(m.ID),
		Email:      m.Email,
		Name:       m.Name,
		ExternalID: m.ExternalID,
	}
}

type UserReader struct {
	db database.UserRepo
}

const LoaderNameUsers = "users"

func (r *UserReader) load(ctx context.Context, keys []string) []*dataloader.Result[*model.User] {
	// TODO (only fetch users requested by keys var)
	users, err := getUsers(ctx, r.db)
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

func getUsers(ctx context.Context, db database.UserRepo) ([]*database.User, error) {
	limit, offset := 100, 0
	users := make([]*database.User, 0)
	for {
		page, _, err := db.GetUsers(ctx, database.Page{
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			return nil, err
		}
		users = append(users, page...)
		if len(page) < limit {
			break
		}
		offset += limit

	}
	return users, nil
}

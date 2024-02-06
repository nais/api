package loader

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
)

type userReader struct {
	db database.UserRepo
}

func (u userReader) getUsers(ctx context.Context, userIDs []uuid.UUID) ([]*model.User, []error) {
	getID := func(obj *model.User) uuid.UUID { return obj.ID }
	return loadModels(ctx, userIDs, u.db.GetUsersByIDs, ToGraphUser, getID)
}

func GetUser(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	return For(ctx).UserLoader.Load(ctx, userID)
}

func ToGraphUser(u *database.User) *model.User {
	return &model.User{
		ID:         u.ID,
		Email:      u.Email,
		Name:       u.Name,
		ExternalID: u.ExternalID,
	}
}

package loader

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
)

type serviceAccountReader struct {
	db database.ServiceAccountRepo
}

func (s serviceAccountReader) getServiceAccounts(ctx context.Context, ids []uuid.UUID) ([]*model.ServiceAccount, []error) {
	getID := func(obj *model.ServiceAccount) uuid.UUID { return obj.ID }
	return loadModels(ctx, ids, s.db.GetServiceAccountsByIDs, ToGraphServiceAccount, getID)
}

func GetServiceAccount(ctx context.Context, id uuid.UUID) (*model.ServiceAccount, error) {
	return For(ctx).ServiceAccountLoader.Load(ctx, id)
}

func ToGraphServiceAccount(m *database.ServiceAccount) *model.ServiceAccount {
	return &model.ServiceAccount{
		ID:   m.ID,
		Name: m.Name,
	}
}

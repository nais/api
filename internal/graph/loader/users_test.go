package loader

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/stretchr/testify/mock"
)

func TestGetUser(t *testing.T) {
	db := database.NewMockDatabase(t)

	ids := make([]uuid.UUID, 10)
	for i := range ids {
		ids[i] = uuid.New()
	}

	db.EXPECT().GetUsersByIDs(mock.Anything, mock.AnythingOfType("[]uuid.UUID")).RunAndReturn(func(_ context.Context, u []uuid.UUID) ([]*database.User, error) {
		ret := make([]*database.User, len(u))
		for i := range ret {
			ret[i] = &database.User{
				User: &gensql.User{
					ID: u[i],
				},
			}
		}
		return ret, nil
	})

	ctx := NewLoaderContext(context.Background(), db)
	wg := sync.WaitGroup{}
	wg.Add(len(ids))
	for _, id := range ids {
		go func(id uuid.UUID) {
			defer wg.Done()
			user, err := GetUser(ctx, id)
			if err != nil {
				t.Error(err)
			}
			uid, err := user.ID.AsUUID()
			if err != nil {
				t.Error(err)
			}
			if uid != id {
				t.Errorf("expected id %s, got %s", id, user.ID)
			}
		}(id)
	}
	wg.Wait()
}

func TestGetUser_WithMissing(t *testing.T) {
	db := database.NewMockDatabase(t)

	ids := make([]uuid.UUID, 5)
	for i := range ids {
		ids[i] = uuid.New()
	}

	tests := []struct {
		id  uuid.UUID
		err error
	}{
		{ids[0], nil},
		{uuid.New(), pgx.ErrNoRows},
		{ids[1], nil},
		{ids[2], nil},
		{ids[3], nil},
		{ids[4], nil},
		{uuid.New(), pgx.ErrNoRows},
	}

	db.EXPECT().GetUsersByIDs(mock.Anything, mock.AnythingOfType("[]uuid.UUID")).RunAndReturn(func(_ context.Context, _ []uuid.UUID) ([]*database.User, error) {
		ret := make([]*database.User, len(ids))
		for i := range ret {
			ret[i] = &database.User{
				User: &gensql.User{
					ID: ids[i],
				},
			}
		}
		return ret, nil
	})

	ctx := NewLoaderContext(context.Background(), db)
	wg := sync.WaitGroup{}
	wg.Add(len(tests))
	for _, tc := range tests {
		go func(id uuid.UUID, expectedErr error) {
			defer wg.Done()
			user, err := GetUser(ctx, id)
			if err != nil {
				if err != expectedErr {
					t.Errorf("expected error %v, got %v", expectedErr, err)
				}
			}
			if expectedErr != nil {
				if user != nil {
					t.Errorf("expected nil user, got %v", user)
				}
			} else {
				uid, err := user.ID.AsUUID()
				if err != nil {
					t.Error(err)
				}
				if uid != id {
					t.Errorf("expected id %s, got %s", id, user.ID)
				}
			}
		}(tc.id, tc.err)
	}
	wg.Wait()
}

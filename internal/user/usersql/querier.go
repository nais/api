// Code generated by sqlc. DO NOT EDIT.

package usersql

import (
	"context"

	"github.com/google/uuid"
)

type Querier interface {
	Count(ctx context.Context) (int64, error)
	CountMemberships(ctx context.Context, userID uuid.UUID) (int64, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByExternalID(ctx context.Context, externalID string) (*User, error)
	GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*User, error)
	List(ctx context.Context, arg ListParams) ([]*User, error)
	ListGCPGroupsForUser(ctx context.Context, userID uuid.UUID) ([]string, error)
	ListMemberships(ctx context.Context, arg ListMembershipsParams) ([]*ListMembershipsRow, error)
	Update(ctx context.Context, arg UpdateParams) (*User, error)
}

var _ Querier = (*Queries)(nil)

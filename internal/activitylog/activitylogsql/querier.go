// Code generated by sqlc. DO NOT EDIT.

package activitylogsql

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/slug"
)

type Querier interface {
	CountForResource(ctx context.Context, arg CountForResourceParams) (int64, error)
	CountForTeam(ctx context.Context, teamSlug *slug.Slug) (int64, error)
	CountForTeamByResource(ctx context.Context, arg CountForTeamByResourceParams) (int64, error)
	Create(ctx context.Context, arg CreateParams) error
	Get(ctx context.Context, id uuid.UUID) (*ActivityLogEntry, error)
	ListByIDs(ctx context.Context, ids []uuid.UUID) ([]*ActivityLogEntry, error)
	ListForResource(ctx context.Context, arg ListForResourceParams) ([]*ActivityLogEntry, error)
	ListForTeam(ctx context.Context, arg ListForTeamParams) ([]*ActivityLogEntry, error)
	ListForTeamByResource(ctx context.Context, arg ListForTeamByResourceParams) ([]*ActivityLogEntry, error)
}

var _ Querier = (*Queries)(nil)
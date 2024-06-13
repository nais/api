package database

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/database/gensql"
	"k8s.io/utils/ptr"
)

type UsersyncRepo interface {
	CreateUsersyncRun(ctx context.Context, id uuid.UUID, startedAt, finishedAt time.Time, err error) error
	GetUsersyncRuns(ctx context.Context, p Page) ([]*UsersyncRun, int, error)
}

var _ UsersyncRepo = (*database)(nil)

type UsersyncRun struct {
	*gensql.UsersyncRun
}

func (d *database) CreateUsersyncRun(ctx context.Context, id uuid.UUID, startedAt, finishedAt time.Time, err error) error {
	var s, f pgtype.Timestamptz
	_ = s.Scan(startedAt)
	_ = f.Scan(finishedAt)

	var errorMsg *string
	if err != nil {
		errorMsg = ptr.To(err.Error())
	}

	return d.querier.CreateUsersyncRun(ctx, gensql.CreateUsersyncRunParams{
		ID:         id,
		StartedAt:  s,
		FinishedAt: f,
		Error:      errorMsg,
	})
}

func (d *database) GetUsersyncRuns(ctx context.Context, p Page) ([]*UsersyncRun, int, error) {
	runs, err := d.querier.GetUsersyncRuns(ctx, gensql.GetUsersyncRunsParams{
		Offset: int32(p.Offset),
		Limit:  int32(p.Limit),
	})
	if err != nil {
		return nil, 0, err
	}

	ret := make([]*UsersyncRun, len(runs))
	for i, r := range runs {
		ret[i] = &UsersyncRun{UsersyncRun: r}
	}

	total, err := d.querier.GetUsersyncRunsCount(ctx)
	if err != nil {
		return nil, 0, err
	}

	return ret, int(total), nil
}

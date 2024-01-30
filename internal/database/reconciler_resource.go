package database

import (
	"context"

	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

type ReconcilerResource struct {
	*gensql.ReconcilerResource
}

type ReconcilerResourceRepo interface {
	UpsertReconcilerResource(ctx context.Context, reconcilerName string, teamSlug slug.Slug, key, value string, metadata []byte) (*ReconcilerResource, error)
	GetReconcilerResourcesByKey(ctx context.Context, reconcilerName string, teamSlug slug.Slug, key string, offset, limit int) (ret []*ReconcilerResource, total int, err error)
	GetReconcilerResources(ctx context.Context, reconcilerName string, teamSlug *slug.Slug, offset, limit int) ([]*ReconcilerResource, error)
}

func (d *database) GetReconcilerResources(ctx context.Context, reconcilerName string, teamSlug *slug.Slug, offset, limit int) ([]*ReconcilerResource, error) {
	var res []*gensql.ReconcilerResource
	var err error
	if teamSlug != nil {
		res, err = d.querier.GetReconcilerResourcesForReconcilerAndTeam(ctx, gensql.GetReconcilerResourcesForReconcilerAndTeamParams{
			ReconcilerName: reconcilerName,
			TeamSlug:       *teamSlug,
			Offset:         int32(offset),
			Limit:          int32(limit),
		})
	} else {
		res, err = d.querier.GetReconcilerResourcesForReconciler(ctx, gensql.GetReconcilerResourcesForReconcilerParams{
			ReconcilerName: reconcilerName,
			Offset:         int32(offset),
			Limit:          int32(limit),
		})
	}
	if err != nil {
		return nil, err
	}

	var ret []*ReconcilerResource
	for _, r := range res {
		ret = append(ret, &ReconcilerResource{r})
	}

	return ret, nil
}

func (d *database) UpsertReconcilerResource(ctx context.Context, reconcilerName string, teamSlug slug.Slug, key, value string, metadata []byte) (*ReconcilerResource, error) {
	res, err := d.querier.UpsertReconcilerResource(ctx, gensql.UpsertReconcilerResourceParams{
		ReconcilerName: reconcilerName,
		TeamSlug:       teamSlug,
		Name:           key,
		Value:          value,
		Metadata:       metadata,
	})
	if err != nil {
		return nil, err
	}

	return &ReconcilerResource{res}, nil
}

func (d *database) GetReconcilerResourcesByKey(ctx context.Context, reconcilerName string, teamSlug slug.Slug, key string, offset, limit int) ([]*ReconcilerResource, int, error) {
	res, err := d.querier.GetReconcilerResourceByKey(ctx, gensql.GetReconcilerResourceByKeyParams{
		ReconcilerName: reconcilerName,
		TeamSlug:       teamSlug,
		Name:           key,
		Offset:         int32(offset),
		Limit:          int32(limit),
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := d.querier.GetReconcilerResourceByKeyTotal(ctx, gensql.GetReconcilerResourceByKeyTotalParams{
		ReconcilerName: reconcilerName,
		TeamSlug:       teamSlug,
		Name:           key,
	})
	if err != nil {
		return nil, 0, err
	}

	ret := make([]*ReconcilerResource, len(res))
	for i, r := range res {
		ret[i] = &ReconcilerResource{r}
	}

	return ret, int(total), nil
}

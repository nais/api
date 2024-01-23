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
	CreateReconcilerResource(ctx context.Context, reconcilerName string, teamSlug slug.Slug, key, value string, metadata []byte) (*ReconcilerResource, error)
	GetReconcilerResources(ctx context.Context, reconcilerName string, teamSlug *slug.Slug, offset, limit int) ([]*ReconcilerResource, error)
}

func (d *database) GetReconcilerResources(ctx context.Context, reconcilerName string, teamSlug *slug.Slug, offset, limit int) ([]*ReconcilerResource, error) {
	var res []*gensql.ReconcilerResource
	var err error
	if teamSlug != nil {
		res, err = d.querier.GetReconcilerResourcesForReconcilerAndTeam(ctx, reconcilerName, *teamSlug, int32(offset), int32(limit))
	} else {
		res, err = d.querier.GetReconcilerResourcesForReconciler(ctx, reconcilerName, int32(offset), int32(limit))
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

func (d *database) CreateReconcilerResource(ctx context.Context, reconcilerName string, teamSlug slug.Slug, key, value string, metadata []byte) (*ReconcilerResource, error) {
	res, err := d.querier.CreateReconcilerResource(ctx, reconcilerName, teamSlug, key, value, metadata)
	if err != nil {
		return nil, err
	}

	return &ReconcilerResource{res}, nil
}

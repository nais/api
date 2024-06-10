package bigquery

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/graph/apierror"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func (c *Client) BigQueryDatasets(teamSlug slug.Slug) ([]*model.BigQueryDataset, error) {
	bigQueryDatasetListOpsCounter.Add(context.Background(), 1)
	ret := make([]*model.BigQueryDataset, 0)

	for env, infs := range c.informers {
		inf := infs.BigQuery
		if inf == nil {
			continue
		}

		objs, err := inf.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			bigQueryDatasetListErrorCounter.Add(context.Background(), 1)
			return nil, fmt.Errorf("listing bigquerydatasets: %w", err)
		}

		for _, obj := range objs {
			bqs, err := model.ToBigQueryDataset(obj.(*unstructured.Unstructured), env)
			if err != nil {
				bigQueryDatasetListErrorCounter.Add(context.Background(), 1)
				return nil, fmt.Errorf("converting to bigquerydataset: %w", err)
			}

			ret = append(ret, bqs)
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})

	return ret, nil
}

func (c *Client) BigQueryDataset(env string, slug slug.Slug, name string) (*model.BigQueryDataset, error) {
	bigQueryDatasetOpsCounter.Add(context.Background(), 1)
	inf, exists := c.informers[env]
	if !exists {
		bigQueryDatasetErrorCounter.Add(context.Background(), 1)
		return nil, fmt.Errorf("unknown env: %q", env)
	}

	if inf.BigQuery == nil {
		bigQueryDatasetErrorCounter.Add(context.Background(), 1)

		return nil, apierror.Errorf("bigQueryDataset informer not supported in env: %q", env)
	}

	obj, err := inf.BigQuery.Lister().ByNamespace(string(slug)).Get(name)
	if err != nil {
		bigQueryDatasetErrorCounter.Add(context.Background(), 1)

		return nil, fmt.Errorf("get bigQueryDataset: %w", err)
	}

	return model.ToBigQueryDataset(obj.(*unstructured.Unstructured), env)
}

func (c *Client) CostForBiqQueryDataset(ctx context.Context, env string, teamSlug slug.Slug, ownerName string) float64 {
	bigQueryDatasetCostOpsCounter.Add(context.Background(), 1)
	cost := 0.0

	now := time.Now()
	var from, to pgtype.Date
	_ = to.Scan(now)
	_ = from.Scan(now.AddDate(0, 0, -30))

	if sum, err := c.db.CostForInstance(ctx, "BigQuery", from, to, teamSlug, ownerName, env); err != nil {
		bigQueryDatasetCostOpsErrorCounter.Add(context.Background(), 1)
		c.log.WithError(err).Errorf("fetching cost")
	} else {
		cost = float64(sum)
	}

	return cost
}

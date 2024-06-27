package opensearch

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

func (c *Client) OpenSearch(ctx context.Context, teamSlug slug.Slug) ([]*model.OpenSearch, *model.OpenSearchMetrics, error) {
	ret := make([]*model.OpenSearch, 0)
	OpensearchListOpsCounter.Add(ctx, 1)
	for env, infs := range c.informers {
		inf := infs.OpenSearch
		if inf == nil {
			continue
		}

		objs, err := inf.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			OpensearchListErrorCounter.Add(ctx, 1)
			return nil, nil, fmt.Errorf("listing OpenSearches: %w", err)
		}

		for _, obj := range objs {
			openSearch, err := model.ToOpenSearch(obj.(*unstructured.Unstructured), env)
			if err != nil {
				OpensearchListErrorCounter.Add(ctx, 1)
				return nil, nil, fmt.Errorf("converting to OpenSearch: %w", err)
			}

			ret = append(ret, openSearch)
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})

	metrics := &model.OpenSearchMetrics{
		Cost: c.CostForOpenSearch(ctx, teamSlug),
	}

	return ret, metrics, nil
}

func (c *Client) OpenSearchInstance(ctx context.Context, env string, teamSlug slug.Slug, openSearchName string) (*model.OpenSearch, error) {
	inf, exists := c.informers[env]
	OpensearchOpsCounter.Add(ctx, 1)
	if !exists {
		OpensearchErrorCounter.Add(ctx, 1)
		return nil, fmt.Errorf("unknown env: %q", env)
	}

	if inf.OpenSearch == nil {
		OpensearchErrorCounter.Add(ctx, 1)
		return nil, apierror.Errorf("openSearch informer not supported in env: %q", env)
	}

	obj, err := inf.OpenSearch.Lister().ByNamespace(string(teamSlug)).Get(openSearchName)
	if err != nil {
		OpensearchErrorCounter.Add(ctx, 1)
		return nil, fmt.Errorf("get openSearch: %w", err)
	}

	ret, err := model.ToOpenSearch(obj.(*unstructured.Unstructured), env)
	if err != nil {
		OpensearchErrorCounter.Add(ctx, 1)
		return nil, err
	}

	return ret, nil
}

func (c *Client) CostForOpenSearchInstance(ctx context.Context, env string, teamSlug slug.Slug, ownerName string) float64 {
	OpensearchCostOpsCounter.Add(ctx, 1)
	cost := 0.0
	now := time.Now()
	var from, to pgtype.Date
	_ = to.Scan(now)
	_ = from.Scan(now.AddDate(0, 0, -30))

	if sum, err := c.db.CostForInstance(ctx, "OpenSearch", from, to, teamSlug, ownerName, env); err != nil {
		OpensearchCostErrorCounter.Add(ctx, 1)
		c.log.WithError(err).Errorf("fetching cost")
	} else {
		cost = float64(sum)
	}
	return cost
}

func (c *Client) CostForOpenSearch(ctx context.Context, teamSlug slug.Slug) float64 {
	OpensearchCostOpsCounter.Add(ctx, 1)

	cost := 0.0

	now := time.Now()
	var from, to pgtype.Date
	_ = to.Scan(now)
	_ = from.Scan(now.AddDate(0, 0, -30))

	if sum, err := c.db.CostForTeam(ctx, "OpenSearch", from, to, teamSlug); err != nil {
		OpensearchCostErrorCounter.Add(ctx, 1)
		c.log.WithError(err).Errorf("fetching cost")
	} else {
		cost = float64(sum)
	}

	return cost
}

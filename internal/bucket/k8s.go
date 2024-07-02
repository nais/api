package bucket

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

func (c *Client) Buckets(ctx context.Context, teamSlug slug.Slug) ([]*model.Bucket, *model.BucketsMetrics, error) {
	ret := make([]*model.Bucket, 0)
	bucketListOpsCounter.Add(ctx, 1)
	for env, infs := range c.informers {
		inf := infs.Bucket
		if inf == nil {
			continue
		}

		objs, err := inf.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			bucketListErrorCounter.Add(ctx, 1)
			return nil, nil, fmt.Errorf("listing Buckets: %w", err)
		}

		for _, obj := range objs {
			bucket, err := model.ToBucket(obj.(*unstructured.Unstructured), env)
			if err != nil {
				bucketListErrorCounter.Add(ctx, 1)
				return nil, nil, fmt.Errorf("converting to Bucket: %w", err)
			}

			ret = append(ret, bucket)
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})

	metrics := &model.BucketsMetrics{
		Cost: c.CostForBuckets(ctx, teamSlug),
	}

	return ret, metrics, nil
}

func (c *Client) Bucket(ctx context.Context, env string, teamSlug slug.Slug, bucketName string) (*model.Bucket, error) {
	bucketOpsCounter.Add(ctx, 1)
	inf, exists := c.informers[env]
	if !exists {
		bucketErrorCounter.Add(ctx, 1)
		return nil, fmt.Errorf("unknown env: %q", env)
	}

	if inf.Bucket == nil {
		bucketErrorCounter.Add(ctx, 1)
		return nil, apierror.Errorf("bucket informer not supported in env: %q", env)
	}

	obj, err := inf.Bucket.Lister().ByNamespace(string(teamSlug)).Get(bucketName)
	if err != nil {
		bucketErrorCounter.Add(ctx, 1)
		return nil, fmt.Errorf("get bucket: %w", err)
	}

	return model.ToBucket(obj.(*unstructured.Unstructured), env)
}

func (c *Client) CostForBuckets(ctx context.Context, teamSlug slug.Slug) float64 {
	bucketCostOpsCounter.Add(ctx, 1)

	cost := 0.0

	now := time.Now()
	var from, to pgtype.Date
	_ = to.Scan(now)
	_ = from.Scan(now.AddDate(0, 0, -30))

	if sum, err := c.db.CostForTeam(ctx, "Cloud Storage", from, to, teamSlug); err != nil {
		bucketCostOpsErrorCounter.Add(ctx, 1)
		c.log.WithError(err).Errorf("fetching cost")
	} else {
		cost = float64(sum)
	}

	return cost
}

package bucket

import (
	"context"
	"fmt"
	"sort"

	"github.com/nais/api/internal/graph/apierror"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func (c *Client) Buckets(teamSlug slug.Slug) ([]*model.Bucket, error) {
	ret := make([]*model.Bucket, 0)
	bucketListOpsCounter.Add(context.Background(), 1)
	for env, infs := range c.informers {
		inf := infs.Bucket
		if inf == nil {
			continue
		}

		objs, err := inf.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			bucketListErrorCounter.Add(context.Background(), 1)
			return nil, fmt.Errorf("listing Buckets: %w", err)
		}

		for _, obj := range objs {
			bucket, err := model.ToBucket(obj.(*unstructured.Unstructured), env)
			if err != nil {
				bucketListErrorCounter.Add(context.Background(), 1)
				return nil, fmt.Errorf("converting to Bucket: %w", err)
			}

			ret = append(ret, bucket)
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})

	return ret, nil
}

func (c *Client) Bucket(env string, teamSlug slug.Slug, bucketName string) (*model.Bucket, error) {
	bucketOpsCounter.Add(context.Background(), 1)
	inf, exists := c.informers[env]
	if !exists {
		bucketErrorCounter.Add(context.Background(), 1)
		return nil, fmt.Errorf("unknown env: %q", env)
	}

	if inf.Bucket == nil {
		bucketErrorCounter.Add(context.Background(), 1)
		return nil, apierror.Errorf("bucket informer not supported in env: %q", env)
	}

	obj, err := inf.Bucket.Lister().ByNamespace(string(teamSlug)).Get(bucketName)
	if err != nil {
		bucketErrorCounter.Add(context.Background(), 1)
		return nil, fmt.Errorf("get bucket: %w", err)
	}

	return model.ToBucket(obj.(*unstructured.Unstructured), env)
}

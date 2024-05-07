package bucket

import (
	"fmt"
	"sort"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func (c *Client) Buckets(teamSlug slug.Slug) ([]*model.Bucket, error) {
	ret := make([]*model.Bucket, 0)

	for env, infs := range c.informers {
		inf := infs.BucketInformer
		if inf == nil {
			continue
		}

		objs, err := inf.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("listing Buckets: %w", err)
		}

		for _, obj := range objs {
			bucket, err := model.ToBucket(obj.(*unstructured.Unstructured), env)
			if err != nil {
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

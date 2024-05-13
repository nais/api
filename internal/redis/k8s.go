package redis

import (
	"fmt"
	"sort"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func (c *Client) Redis(teamSlug slug.Slug) ([]*model.Redis, error) {
	ret := make([]*model.Redis, 0)

	for env, infs := range c.informers {
		inf := infs.Redis
		if inf == nil {
			continue
		}

		objs, err := inf.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("listing Rediss: %w", err)
		}

		for _, obj := range objs {
			redis, err := model.ToRedis(obj.(*unstructured.Unstructured), env)
			if err != nil {
				return nil, fmt.Errorf("converting to Redis: %w", err)
			}

			ret = append(ret, redis)
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})

	return ret, nil
}

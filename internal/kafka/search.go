package kafka

import (
	"context"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/search"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func (c *Client) SupportsSearchFilter(filter *model.SearchFilter) bool {
	if emptyFilter(filter) {
		return true
	}

	return *filter.Type == model.SearchTypeKafkatopic
}

func (c *Client) Search(ctx context.Context, q string, filter *model.SearchFilter) []*search.Result {
	ret := make([]*search.Result, 0)

	if c.db == nil {
		c.log.Warnf("database not set, unable to perform search")
		return ret
	}

	for env, infs := range c.informers {
		if infs.KafkaTopic == nil {
			continue
		}

		redisInstances, err := infs.KafkaTopic.Lister().List(labels.Everything())
		if err != nil {
			c.log.WithError(err).Error("listing KafkaTopic instances")
			return nil
		}

		for _, obj := range redisInstances {
			u := obj.(*unstructured.Unstructured)
			rank := search.Match(q, u.GetName())
			if rank == -1 {
				continue
			}

			redisInstance, err := model.ToKafkaTopic(u, env)
			if err != nil {
				c.log.WithError(err).Error("converting KafkaTopic instances")
				return nil
			} else if ok, _ := c.db.TeamExists(ctx, redisInstance.GQLVars.TeamSlug); !ok {
				continue
			}

			ret = append(ret, &search.Result{
				Node: redisInstance,
				Rank: rank,
			})
		}
	}
	return ret
}

func emptyFilter(filter *model.SearchFilter) bool {
	return filter == nil || filter.Type == nil
}

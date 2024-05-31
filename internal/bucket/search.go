package bucket

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

	return *filter.Type == model.SearchTypeBucket
}

func (c *Client) Search(ctx context.Context, q string, filter *model.SearchFilter) []*search.Result {
	ret := make([]*search.Result, 0)

	if c.db == nil {
		c.log.Warnf("database not set, unable to perform search")
		return ret
	}

	for env, infs := range c.informers {
		if infs.Bucket == nil {
			continue
		}

		buckets, err := infs.Bucket.Lister().List(labels.Everything())
		if err != nil {
			c.log.WithError(err).Error("listing Buckets")
			return nil
		}

		for _, obj := range buckets {
			u := obj.(*unstructured.Unstructured)
			rank := search.Match(q, u.GetName())
			if rank == -1 {
				continue
			}

			bucket, err := model.ToBucket(u, env)
			if err != nil {
				c.log.WithError(err).Error("converting Bucket")
				return nil
			} else if ok, _ := c.db.TeamExists(ctx, bucket.GQLVars.TeamSlug); !ok {
				continue
			}

			ret = append(ret, &search.Result{
				Node: bucket,
				Rank: rank,
			})
		}
	}
	return ret
}

func emptyFilter(filter *model.SearchFilter) bool {
	return filter == nil || filter.Type == nil
}

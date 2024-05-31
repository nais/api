package opensearch

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

	return *filter.Type == model.SearchTypeOpensearch
}

func (c *Client) Search(ctx context.Context, q string, filter *model.SearchFilter) []*search.Result {
	ret := make([]*search.Result, 0)

	if c.db == nil {
		c.log.Warnf("database not set, unable to perform search")
		return ret
	}

	for env, infs := range c.informers {
		if infs.OpenSearch == nil {
			continue
		}

		osInstances, err := infs.OpenSearch.Lister().List(labels.Everything())
		if err != nil {
			c.log.WithError(err).Error("listing OpenSearch instances")
			return nil
		}

		for _, obj := range osInstances {
			u := obj.(*unstructured.Unstructured)
			rank := search.Match(q, u.GetName())
			if rank == -1 {
				continue
			}

			openSearchInstance, err := model.ToOpenSearch(u, env)
			if err != nil {
				c.log.WithError(err).Error("converting OpenSearch instances")
				return nil
			} else if ok, _ := c.db.TeamExists(ctx, openSearchInstance.GQLVars.TeamSlug); !ok {
				continue
			}

			ret = append(ret, &search.Result{
				Node: openSearchInstance,
				Rank: rank,
			})
		}
	}
	return ret
}

func emptyFilter(filter *model.SearchFilter) bool {
	return filter == nil || filter.Type == nil
}

package opensearch

import (
	"fmt"
	"sort"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func (c *Client) OpenSearch(teamSlug slug.Slug) ([]*model.OpenSearch, error) {
	ret := make([]*model.OpenSearch, 0)

	for env, infs := range c.informers {
		inf := infs.OpenSearch
		if inf == nil {
			continue
		}

		apps, err := infs.App.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			c.log.WithError(err).Errorf("listing team apps")
		}
		naisJobs, err := infs.Naisjob.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			c.log.WithError(err).Errorf("listing team jobs")
		}

		objs, err := inf.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("listing OpenSearches: %w", err)
		}

		for _, obj := range objs {
			o := obj.(*unstructured.Unstructured)

			access, err := getAccess(apps, naisJobs, o.GetName())
			if err != nil {
				return nil, fmt.Errorf("getting access for openSearch instance: %w", err)
			}

			openSearch, err := model.ToOpenSearch(obj.(*unstructured.Unstructured), access, env)
			if err != nil {
				return nil, fmt.Errorf("converting to OpenSearch: %w", err)
			}

			ret = append(ret, openSearch)
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})

	return ret, nil
}

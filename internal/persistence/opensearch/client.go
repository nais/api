package opensearch

import (
	"context"
	"fmt"
	"sort"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/slug"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

type client struct {
	informers k8s.ClusterInformers
}

func (c client) getOpenSearches(ctx context.Context, ids []resourceIdentifier) ([]*OpenSearch, error) {
	ret := make([]*OpenSearch, 0)
	for _, id := range ids {
		v, err := c.getOpenSearch(ctx, id.environment, id.namespace, id.name)
		if err != nil {
			continue
		}
		ret = append(ret, v)
	}
	return ret, nil
}

func (c client) getOpenSearchesForTeam(_ context.Context, teamSlug slug.Slug) ([]*OpenSearch, error) {
	ret := make([]*OpenSearch, 0)

	for env, infs := range c.informers {
		inf := infs.OpenSearch
		if inf == nil {
			continue
		}

		objs, err := inf.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("listing opensearch instances: %w", err)
		}

		for _, obj := range objs {
			bqs, err := toOpenSearch(obj.(*unstructured.Unstructured), env)
			if err != nil {
				return nil, fmt.Errorf("converting to opensearch instasnce: %w", err)
			}

			ret = append(ret, bqs)
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})

	return ret, nil
}

func (c client) getOpenSearch(_ context.Context, env string, namespace string, name string) (*OpenSearch, error) {
	inf, exists := c.informers[env]
	if !exists {
		return nil, fmt.Errorf("unknown env: %q", env)
	}

	if inf.OpenSearch == nil {
		return nil, apierror.Errorf("OpenSearch informer not supported in env: %q", env)
	}

	obj, err := inf.OpenSearch.Lister().ByNamespace(namespace).Get(name)
	if err != nil {
		return nil, fmt.Errorf("get OpenSearch: %w", err)
	}

	return toOpenSearch(obj.(*unstructured.Unstructured), env)
}

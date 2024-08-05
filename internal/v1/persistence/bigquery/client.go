package bigquery

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

func (l client) getBigQueryDatasets(ctx context.Context, ids []resourceIdentifier) ([]*BigQueryDataset, error) {
	ret := make([]*BigQueryDataset, 0)
	for _, id := range ids {
		v, err := l.getBigQueryDataset(ctx, id.environment, id.namespace, id.name)
		if err != nil {
			continue
		}
		ret = append(ret, v)
	}
	return ret, nil
}

func (l client) getBigQueryDatasetsForTeam(_ context.Context, teamSlug slug.Slug) ([]*BigQueryDataset, error) {
	ret := make([]*BigQueryDataset, 0)

	for env, infs := range l.informers {
		inf := infs.BigQuery
		if inf == nil {
			continue
		}

		objs, err := inf.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("listing bigquerydatasets: %w", err)
		}

		for _, obj := range objs {
			model, err := toBigQueryDataset(obj.(*unstructured.Unstructured), env)
			if err != nil {
				return nil, fmt.Errorf("converting to bigquerydataset: %w", err)
			}

			ret = append(ret, model)
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})

	return ret, nil
}

func (l client) getBigQueryDataset(_ context.Context, env string, namespace string, name string) (*BigQueryDataset, error) {
	inf, exists := l.informers[env]
	if !exists {
		return nil, fmt.Errorf("unknown env: %q", env)
	}

	if inf.BigQuery == nil {
		return nil, apierror.Errorf("bigQueryDataset informer not supported in env: %q", env)
	}

	obj, err := inf.BigQuery.Lister().ByNamespace(namespace).Get(name)
	if err != nil {
		return nil, fmt.Errorf("get bigQueryDataset: %w", err)
	}

	return toBigQueryDataset(obj.(*unstructured.Unstructured), env)
}

package bigquery

import (
	"context"
	"sort"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
)

type client struct {
	watcher *watcher.Watcher[*BigQueryDataset]
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
	objs := l.watcher.GetByNamespace(teamSlug.String())

	ret := watcher.Objects(objs)

	sort.Slice(ret, func(i, j int) bool {
		return ret[i].GetName() < ret[j].GetName()
	})

	return ret, nil
}

func (l client) getBigQueryDataset(_ context.Context, env string, namespace string, name string) (*BigQueryDataset, error) {
	return l.watcher.Get(env, namespace, name)
}

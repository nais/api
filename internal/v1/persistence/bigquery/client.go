package bigquery

import (
	"context"

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

func (l client) getBigQueryDataset(_ context.Context, env, namespace, name string) (*BigQueryDataset, error) {
	return l.watcher.Get(env, namespace, name)
}

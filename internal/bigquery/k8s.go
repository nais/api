package bigquery

import (
	"fmt"
	"sort"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func (c *Client) BigQueryDatasets(teamSlug slug.Slug) ([]*model.BigQueryDataset, error) {
	ret := make([]*model.BigQueryDataset, 0)

	for env, infs := range c.informers {
		inf := infs.BigQuery
		if inf == nil {
			continue
		}

		objs, err := inf.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("listing bigquerydatasets: %w", err)
		}

		for _, obj := range objs {
			bqs, err := model.ToBigQueryDataset(obj.(*unstructured.Unstructured), env)
			if err != nil {
				return nil, fmt.Errorf("converting to bigquerydataset: %w", err)
			}

			ret = append(ret, bqs)
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})

	return ret, nil
}

package k8s

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

func (c *Client) Persistence(ctx context.Context, workload model.WorkloadBase) ([]model.Persistence, error) {
	cluster := workload.Env.Name
	teamSlug := workload.GQLVars.Team
	ret := make([]model.Persistence, 0)

	req, err := labels.NewRequirement("app", selection.Equals, []string{workload.Name})
	if err != nil {
		return nil, c.error(ctx, err, "creating label selector")
	}

	byAppLabel := labels.NewSelector().Add(*req)

	if inf := c.informers[cluster].Bucket; inf != nil {
		buckets, err := inf.Lister().ByNamespace(string(teamSlug)).List(byAppLabel)
		if err != nil {
			return nil, fmt.Errorf("listing buckets: %w", err)
		}
		for _, bucket := range buckets {
			b, err := model.ToBucket(bucket.(*unstructured.Unstructured), cluster)
			if err != nil {
				return nil, fmt.Errorf("converting bucket: %w", err)
			}
			ret = append(ret, b)
		}
	}

	if inf := c.informers[cluster].BigQuery; inf != nil {
		bqs, err := inf.Lister().ByNamespace(string(teamSlug)).List(byAppLabel)
		if err != nil {
			return nil, fmt.Errorf("listing bigquerydatasets: %w", err)
		}
		for _, bq := range bqs {
			b, err := model.ToBigQueryDataset(bq.(*unstructured.Unstructured), cluster)
			if err != nil {
				return nil, fmt.Errorf("converting bigQueryDataset: %w", err)
			}
			ret = append(ret, b)
		}
	}

	if inf := c.informers[cluster].Redis; inf != nil {
		for _, redis := range workload.GQLVars.Spec.Redis {
			obj, err := inf.Lister().ByNamespace(string(teamSlug)).Get("redis-" + string(teamSlug) + "-" + redis.Instance)
			if err != nil {
				return nil, fmt.Errorf("getting redis: %w", err)
			}

			r, err := model.ToRedis(obj.(*unstructured.Unstructured), cluster)
			if err != nil {
				return nil, fmt.Errorf("converting to redis: %w", err)
			}

			ret = append(ret, r)
		}
	}

	if inf := c.informers[cluster].SqlInstance; inf != nil {
		objs, err := inf.Lister().ByNamespace(string(teamSlug)).List(byAppLabel)
		if err != nil {
			return nil, fmt.Errorf("listing SQL instances: %w", err)
		}
		for _, obj := range objs {
			o, err := model.ToSqlInstance(obj.(*unstructured.Unstructured), cluster)
			if err != nil {
				return nil, fmt.Errorf("converting SQL instance: %w", err)
			}
			ret = append(ret, o)
		}
	}

	if inf := c.informers[cluster].OpenSearch; inf != nil {
		if workload.GQLVars.Spec.OpenSearch != nil {
			obj, err := inf.Lister().ByNamespace(string(teamSlug)).Get("opensearch-" + string(teamSlug) + "-" + workload.GQLVars.Spec.OpenSearch.Instance)
			if err != nil {
				return nil, fmt.Errorf("getting OpenSearch: %w", err)
			}

			if obj != nil {
				o, err := model.ToOpenSearch(obj.(*unstructured.Unstructured), cluster)
				if err != nil {
					return nil, fmt.Errorf("converting OpenSearch: %w", err)
				}

				ret = append(ret, o)
			}
		}
	}

	if inf := c.informers[cluster].KafkaTopic; inf != nil {
		objs, err := inf.Lister().ByNamespace(string(teamSlug)).List(byAppLabel)
		if err != nil {
			return nil, fmt.Errorf("listing KafkaTopic instances: %w", err)
		}
		for _, obj := range objs {
			o, err := model.ToKafkaTopic(obj.(*unstructured.Unstructured), cluster)
			if err != nil {
				return nil, fmt.Errorf("converting KafkaTopic instance: %w", err)
			}
			ret = append(ret, o)
		}
	}

	return ret, nil
}

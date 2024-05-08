package k8s

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func (c *Client) Persistence(ctx context.Context, workload model.WorkloadBase) ([]model.Persistence, error) {
	cluster := workload.Env.Name
	teamSlug := workload.GQLVars.Team
	topics, err := c.getTopics(ctx, workload.Name, string(teamSlug), cluster)
	if err != nil {
		return nil, c.error(ctx, err, "getting topics")
	}

	ret := make([]model.Persistence, 0)

	if inf := c.informers[cluster].BucketInformer; inf != nil {
		buckets, err := inf.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
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

	spec := workload.GQLVars.Spec

	if spec.GCP != nil {
		// TODO: Use SqlInstance informer for this instead?
		for _, v := range spec.GCP.SqlInstances {
			sqlInstance := model.SQLInstance{}
			if err := convert(v, &sqlInstance); err != nil {
				return nil, fmt.Errorf("converting sqlInstance: %w", err)
			}
			if sqlInstance.Name == "" {
				sqlInstance.Name = workload.Name
			}
			sqlInstance.ID = scalar.SqlInstanceIdent("sqlInstance_" + cluster + "_" + string(teamSlug) + "_" + sqlInstance.GetName())
			ret = append(ret, sqlInstance)
		}

		for _, v := range spec.GCP.BigQueryDatasets {
			bqDataset := model.BigQueryDataset{}
			if err := convert(v, &bqDataset); err != nil {
				return nil, fmt.Errorf("converting bigQueryDataset: %w", err)
			}
			ret = append(ret, bqDataset)
		}
	}

	if spec.OpenSearch != nil {
		os := model.OpenSearch{
			Name:   spec.OpenSearch.Instance,
			Access: spec.OpenSearch.Access,
		}
		ret = append(ret, os)
	}

	if spec.Kafka != nil {
		kafka := model.Kafka{
			Name:    spec.Kafka.Pool,
			Streams: spec.Kafka.Streams,
			Topics:  topics,
		}

		ret = append(ret, kafka)
	}

	if len(spec.Redis) > 0 {
		for _, v := range spec.Redis {
			redis := model.Redis{
				Name:   v.Instance,
				Access: v.Access,
			}
			ret = append(ret, redis)
		}
	}

	if spec.Influx != nil {
		influx := model.InfluxDb{
			Name: spec.Influx.Instance,
		}
		ret = append(ret, influx)
	}
	return ret, nil
}

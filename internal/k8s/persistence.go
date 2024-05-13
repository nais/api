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
	spec := workload.GQLVars.Spec
	ret := make([]model.Persistence, 0)

	if inf := c.informers[cluster].Bucket; inf != nil {
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

	if inf := c.informers[cluster].BigQuery; inf != nil {
		bqs, err := inf.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
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
		redises, err := inf.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("getting redis: %w", err)
		}
		for _, redis := range redises {
			r, err := model.ToRedis(redis.(*unstructured.Unstructured), cluster)
			if err != nil {
				return nil, fmt.Errorf("converting to redis: %w", err)
			}

			// TODO: maybe make this an informer 🙃
			for _, specRedis := range spec.Redis {
				if specRedis.Instance != r.Name {
					continue
				}
				r.Access = specRedis.Access
			}
			ret = append(ret, r)
		}
	}

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

	if spec.Influx != nil {
		influx := model.InfluxDb{
			Name: spec.Influx.Instance,
		}
		ret = append(ret, influx)
	}
	return ret, nil
}

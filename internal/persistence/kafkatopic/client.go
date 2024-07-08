package kafkatopic

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

func (c client) getKafkaTopics(ctx context.Context, ids []resourceIdentifier) ([]*KafkaTopic, error) {
	ret := make([]*KafkaTopic, 0)
	for _, id := range ids {
		v, err := c.getKafkaTopic(ctx, id.environment, id.namespace, id.name)
		if err != nil {
			continue
		}
		ret = append(ret, v)
	}
	return ret, nil
}

func (c client) getKafkaTopicsForTeam(_ context.Context, teamSlug slug.Slug) ([]*KafkaTopic, error) {
	ret := make([]*KafkaTopic, 0)

	for env, infs := range c.informers {
		inf := infs.KafkaTopic
		if inf == nil {
			continue
		}

		objs, err := inf.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("listing Kafka topics: %w", err)
		}

		for _, obj := range objs {
			bqs, err := toKafkaTopic(obj.(*unstructured.Unstructured), env)
			if err != nil {
				return nil, fmt.Errorf("converting to kafka topic: %w", err)
			}

			ret = append(ret, bqs)
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})

	return ret, nil
}

func (c client) getKafkaTopic(_ context.Context, env string, namespace string, name string) (*KafkaTopic, error) {
	inf, exists := c.informers[env]
	if !exists {
		return nil, fmt.Errorf("unknown env: %q", env)
	}

	if inf.KafkaTopic == nil {
		return nil, apierror.Errorf("KafkaTopic informer not supported in env: %q", env)
	}

	obj, err := inf.KafkaTopic.Lister().ByNamespace(namespace).Get(name)
	if err != nil {
		return nil, fmt.Errorf("get KafkaTopic: %w", err)
	}

	return toKafkaTopic(obj.(*unstructured.Unstructured), env)
}

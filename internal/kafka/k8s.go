package kafka

import (
	"fmt"
	"sort"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func (c *Client) Topics(teamSlug slug.Slug) ([]*model.KafkaTopic, error) {
	ret := make([]*model.KafkaTopic, 0)

	for env, infs := range c.informers {
		inf := infs.KafkaTopic
		if inf == nil {
			continue
		}

		objs, err := inf.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("listing KafkaTopics: %w", err)
		}

		for _, obj := range objs {
			redis, err := model.ToKafkaTopic(obj.(*unstructured.Unstructured), env)
			if err != nil {
				return nil, fmt.Errorf("converting to KafkaTopic: %w", err)
			}

			ret = append(ret, redis)
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})

	return ret, nil
}

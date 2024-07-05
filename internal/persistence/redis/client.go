package redis

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

func (c client) getRedisInstances(ctx context.Context, ids []resourceIdentifier) ([]*RedisInstance, error) {
	ret := make([]*RedisInstance, 0)
	for _, id := range ids {
		v, err := c.getRedis(ctx, id.environment, id.namespace, id.name)
		if err != nil {
			continue
		}
		ret = append(ret, v)
	}
	return ret, nil
}

func (c client) getRedisInstancesForTeam(_ context.Context, teamSlug slug.Slug) ([]*RedisInstance, error) {
	ret := make([]*RedisInstance, 0)

	for env, infs := range c.informers {
		inf := infs.Redis
		if inf == nil {
			continue
		}

		objs, err := inf.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("listing redis instances: %w", err)
		}

		for _, obj := range objs {
			bqs, err := toRedisInstance(obj.(*unstructured.Unstructured), env)
			if err != nil {
				return nil, fmt.Errorf("converting to redis instasnce: %w", err)
			}

			ret = append(ret, bqs)
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})

	return ret, nil
}

func (c client) getRedis(_ context.Context, env string, namespace string, name string) (*RedisInstance, error) {
	inf, exists := c.informers[env]
	if !exists {
		return nil, fmt.Errorf("unknown env: %q", env)
	}

	if inf.Redis == nil {
		return nil, apierror.Errorf("Redis informer not supported in env: %q", env)
	}

	obj, err := inf.Redis.Lister().ByNamespace(namespace).Get(name)
	if err != nil {
		return nil, fmt.Errorf("get Redis: %w", err)
	}

	return toRedisInstance(obj.(*unstructured.Unstructured), env)
}

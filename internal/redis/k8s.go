package redis

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func (c *Client) Redis(teamSlug slug.Slug) ([]*model.Redis, error) {
	ret := make([]*model.Redis, 0)

	for env, infs := range c.informers {
		inf := infs.Redis
		if inf == nil {
			continue
		}

		objs, err := inf.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("listing Redis: %w", err)
		}

		for _, obj := range objs {
			redis, err := model.ToRedis(obj.(*unstructured.Unstructured), env)
			if err != nil {
				return nil, fmt.Errorf("converting to Redis: %w", err)
			}

			ret = append(ret, redis)
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})

	return ret, nil
}

func (c *Client) RedisInstance(ctx context.Context, env string, teamSlug slug.Slug, redisName string) (*model.Redis, error) {
	inf, exists := c.informers[env]
	if !exists {
		return nil, fmt.Errorf("unknown env: %q", env)
	}

	if inf.Redis == nil {
		return nil, apierror.Errorf("redis informer not supported in env: %q", env)
	}

	obj, err := inf.Redis.Lister().ByNamespace(string(teamSlug)).Get(redisName)
	if err != nil {
		return nil, fmt.Errorf("get redis: %w", err)
	}

	ret, err := model.ToRedis(obj.(*unstructured.Unstructured), env)
	if err != nil {
		return nil, err
	}

	if ret.GQLVars.OwnerReference != nil {
		cost := c.metrics.CostForRedisInstance(ctx, env, teamSlug, ret.GQLVars.OwnerReference.Name)
		ret.Cost = strconv.FormatFloat(cost, 'f', -1, 64)
	}

	return ret, nil
}

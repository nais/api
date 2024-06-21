package redis

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func (c *Client) Redis(ctx context.Context, teamSlug slug.Slug) ([]*model.Redis, error) {
	RedisListOpsCounter.Add(ctx, 1)
	ret := make([]*model.Redis, 0)

	for env, infs := range c.informers {
		inf := infs.Redis
		if inf == nil {
			continue
		}

		objs, err := inf.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			RedisListErrorCounter.Add(ctx, 1)
			return nil, fmt.Errorf("listing Redis: %w", err)
		}

		for _, obj := range objs {
			redis, err := model.ToRedis(obj.(*unstructured.Unstructured), env)
			if err != nil {
				RedisListErrorCounter.Add(ctx, 1)
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
	RedisOpsCounter.Add(ctx, 1)
	inf, exists := c.informers[env]
	if !exists {
		RedisErrorCounter.Add(ctx, 1)
		return nil, fmt.Errorf("unknown env: %q", env)
	}

	if inf.Redis == nil {
		RedisErrorCounter.Add(ctx, 1)
		return nil, apierror.Errorf("redis informer not supported in env: %q", env)
	}

	obj, err := inf.Redis.Lister().ByNamespace(string(teamSlug)).Get(redisName)
	if err != nil {
		RedisErrorCounter.Add(ctx, 1)
		return nil, fmt.Errorf("get redis: %w", err)
	}

	ret, err := model.ToRedis(obj.(*unstructured.Unstructured), env)
	if err != nil {
		RedisErrorCounter.Add(ctx, 1)

		return nil, err
	}

	return ret, nil
}

func (c *Client) CostForRedisInstance(ctx context.Context, env string, teamSlug slug.Slug, ownerName string) float64 {
	RedisCostOpsCounter.Add(ctx, 1)

	cost := 0.0

	now := time.Now()
	var from, to pgtype.Date
	_ = to.Scan(now)
	_ = from.Scan(now.AddDate(0, 0, -30))

	if sum, err := c.db.CostForInstance(ctx, "Redis", from, to, teamSlug, ownerName, env); err != nil {
		RedisCostErrorCounter.Add(ctx, 1)
		c.log.WithError(err).Errorf("fetching cost")
	} else {
		cost = float64(sum)
	}

	return cost
}

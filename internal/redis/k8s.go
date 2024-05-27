package redis

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

func getAccess(apps []runtime.Object, naisJobs []runtime.Object, redisInstance string) (*model.Access, error) {
	access := &model.Access{}

	for _, a := range apps {
		app := &nais_io_v1alpha1.Application{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(a.(*unstructured.Unstructured).Object, app); err != nil {
			return nil, fmt.Errorf("converting to application: %w", err)
		}

		for _, r := range app.Spec.Redis {
			if r.Instance == redisInstance {
				access.Workloads = append(access.Workloads, model.AccessEntry{
					OwnerReference: &v1.OwnerReference{
						APIVersion: app.APIVersion,
						Kind:       app.Kind,
						Name:       app.Name,
						UID:        app.UID,
					},
					Role: r.Access,
				})
			}
		}
	}

	for _, j := range naisJobs {
		job := &nais_io_v1.Naisjob{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(j.(*unstructured.Unstructured).Object, job); err != nil {
			return nil, fmt.Errorf("converting to job: %w", err)
		}

		for _, r := range job.Spec.Redis {
			if r.Instance == redisInstance {
				access.Workloads = append(access.Workloads, model.AccessEntry{
					OwnerReference: &v1.OwnerReference{
						APIVersion: job.APIVersion,
						Kind:       job.Kind,
						Name:       job.Name,
						UID:        job.UID,
					},
					Role: r.Access,
				})
			}
		}
	}

	return access, nil
}

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

		apps, err := infs.App.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			c.log.WithError(err).Errorf("listing team apps")
		}
		naisJobs, err := infs.Naisjob.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			c.log.WithError(err).Errorf("listing team jobs")
		}

		for _, obj := range objs {
			o := obj.(*unstructured.Unstructured)

			access, err := getAccess(apps, naisJobs, o.GetName())
			if err != nil {
				return nil, fmt.Errorf("getting access for redis instance: %w", err)
			}

			redis, err := model.ToRedis(o, access, env)
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

	apps, err := inf.App.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
	if err != nil {
		c.log.WithError(err).Errorf("listing team apps")
	}
	naisJobs, err := inf.Naisjob.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
	if err != nil {
		c.log.WithError(err).Errorf("listing team jobs")
	}

	access, err := getAccess(apps, naisJobs, redisName)
	if err != nil {
		return nil, fmt.Errorf("getting access for redis instance: %w", err)
	}

	ret, err := model.ToRedis(obj.(*unstructured.Unstructured), access, env)
	if err != nil {
		return nil, err
	}

	if ret.GQLVars.OwnerReference != nil {
		cost := c.metrics.CostForRedisInstance(ctx, env, teamSlug, ret.GQLVars.OwnerReference.Name)
		ret.Cost = strconv.FormatFloat(cost, 'f', -1, 64)
	}

	return ret, nil
}

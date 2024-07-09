package redis

import (
	"context"
	"fmt"
	"sort"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/slug"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
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
			model, err := toRedisInstance(obj.(*unstructured.Unstructured), env)
			if err != nil {
				return nil, fmt.Errorf("converting to redis instance: %w", err)
			}

			ret = append(ret, model)
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

func (c client) getAccessForApplications(environmentName, redisInstanceName string, teamSlug slug.Slug) ([]*RedisInstanceAccess, error) {
	infs, exists := c.informers[environmentName]
	if !exists {
		return nil, fmt.Errorf("unknown environment: %q", environmentName)
	}

	if infs.Redis == nil {
		return nil, apierror.Errorf("Redis informer not supported in environment: %q", environmentName)
	}

	access := make([]*RedisInstanceAccess, 0)
	apps, err := infs.App.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("unable to list applications for team")
	}

	for _, a := range apps {
		app := &nais_io_v1alpha1.Application{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(a.(*unstructured.Unstructured).Object, app); err != nil {
			return nil, fmt.Errorf("converting to application: %w", err)
		}

		for _, r := range app.Spec.Redis {
			if "redis-"+string(teamSlug)+"-"+r.Instance == redisInstanceName {
				access = append(access, &RedisInstanceAccess{
					Access:          r.Access,
					TeamSlug:        teamSlug,
					EnvironmentName: environmentName,
					OwnerReference: &metav1.OwnerReference{
						APIVersion: app.APIVersion,
						Kind:       app.Kind,
						Name:       app.Name,
						UID:        app.UID,
					},
				})
			}
		}

	}

	return access, nil
}

func (c client) getAccessForJobs(environmentName, redisInstanceName string, teamSlug slug.Slug) ([]*RedisInstanceAccess, error) {
	infs, exists := c.informers[environmentName]
	if !exists {
		return nil, fmt.Errorf("unknown environment: %q", environmentName)
	}

	if infs.Redis == nil {
		return nil, apierror.Errorf("Redis informer not supported in environment: %q", environmentName)
	}

	access := make([]*RedisInstanceAccess, 0)
	naisJobs, err := infs.Naisjob.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("unable to list jobs for team")
	}

	for _, j := range naisJobs {
		job := &nais_io_v1.Naisjob{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(j.(*unstructured.Unstructured).Object, job); err != nil {
			return nil, fmt.Errorf("converting to job: %w", err)
		}

		for _, r := range job.Spec.Redis {
			if "redis-"+string(teamSlug)+"-"+r.Instance == redisInstanceName {
				access = append(access, &RedisInstanceAccess{
					Access:          r.Access,
					TeamSlug:        teamSlug,
					EnvironmentName: environmentName,
					OwnerReference: &metav1.OwnerReference{
						APIVersion: job.APIVersion,
						Kind:       job.Kind,
						Name:       job.Name,
						UID:        job.UID,
					},
				})
			}
		}
	}

	return access, nil
}

package sqlinstance

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

func (c client) getInstances(ctx context.Context, ids []identifier) ([]*SQLInstance, error) {
	ret := make([]*SQLInstance, 0)
	for _, id := range ids {
		v, err := c.getInstance(ctx, id.namespace, id.environmentName, id.sqlInstanceName)
		if err != nil {
			continue
		}
		ret = append(ret, v)
	}
	return ret, nil
}

func (c client) getInstance(_ context.Context, namespace string, environmentName string, sqlInstanceName string) (*SQLInstance, error) {
	inf, exists := c.informers[environmentName]
	if !exists {
		return nil, fmt.Errorf("unknown env: %q", environmentName)
	}

	if inf.SqlInstance == nil {
		return nil, apierror.Errorf("SQL instance informer not supported in env: %q", environmentName)
	}

	obj, err := inf.SqlInstance.Lister().ByNamespace(namespace).Get(sqlInstanceName)
	if err != nil {
		return nil, fmt.Errorf("get SQL instance: %w", err)
	}

	return toSQLInstance(obj.(*unstructured.Unstructured), environmentName)
}

func (c client) getInstancesForTeam(_ context.Context, teamSlug slug.Slug) ([]*SQLInstance, error) {
	ret := make([]*SQLInstance, 0)

	for env, infs := range c.informers {
		inf := infs.SqlInstance
		if inf == nil {
			continue
		}

		objs, err := inf.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("listing SQL instances: %w", err)
		}

		for _, obj := range objs {
			model, err := toSQLInstance(obj.(*unstructured.Unstructured), env)
			if err != nil {
				return nil, fmt.Errorf("converting to SQL instance: %w", err)
			}

			ret = append(ret, model)
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})

	return ret, nil
}

func (c client) getDatabases(ctx context.Context, ids []identifier) ([]*SQLDatabase, error) {
	ret := make([]*SQLDatabase, 0)
	for _, id := range ids {
		v, err := c.getDatabase(ctx, id.namespace, id.environmentName, id.sqlInstanceName)
		if err != nil {
			continue
		}

		if v == nil {
			continue
		}

		ret = append(ret, v)
	}
	return ret, nil
}

func (c client) getDatabase(_ context.Context, namespace, environmentName, sqlInstanceName string) (*SQLDatabase, error) {
	inf, exists := c.informers[environmentName]
	if !exists {
		return nil, fmt.Errorf("unknown env: %q", environmentName)
	}

	if inf.SqlDatabase == nil {
		return nil, apierror.Errorf("SQL database informer not supported in env: %q", environmentName)
	}

	objs, err := inf.SqlDatabase.Lister().ByNamespace(namespace).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("get SQL database: %w", err)
	}

	// TODO: Is this really the best way to find which database belongs to an instance?
	for _, obj := range objs {
		if db, err := toSQLDatabase(obj.(*unstructured.Unstructured), environmentName, sqlInstanceName); err != nil {
			return nil, fmt.Errorf("converting SQL database: %w", err)
		} else if db != nil {
			return db, nil
		}
	}

	return nil, nil
}

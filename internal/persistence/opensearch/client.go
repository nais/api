package opensearch

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

func (c client) getOpenSearches(ctx context.Context, ids []resourceIdentifier) ([]*OpenSearch, error) {
	ret := make([]*OpenSearch, 0)
	for _, id := range ids {
		v, err := c.getOpenSearch(ctx, id.environment, id.namespace, id.name)
		if err != nil {
			continue
		}
		ret = append(ret, v)
	}
	return ret, nil
}

func (c client) getOpenSearchesForTeam(_ context.Context, teamSlug slug.Slug) ([]*OpenSearch, error) {
	ret := make([]*OpenSearch, 0)

	for env, infs := range c.informers {
		inf := infs.OpenSearch
		if inf == nil {
			continue
		}

		objs, err := inf.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("listing opensearch instances: %w", err)
		}

		for _, obj := range objs {
			bqs, err := toOpenSearch(obj.(*unstructured.Unstructured), env)
			if err != nil {
				return nil, fmt.Errorf("converting to opensearch instance: %w", err)
			}

			ret = append(ret, bqs)
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})

	return ret, nil
}

func (c client) getOpenSearch(_ context.Context, env string, namespace string, name string) (*OpenSearch, error) {
	inf, exists := c.informers[env]
	if !exists {
		return nil, fmt.Errorf("unknown env: %q", env)
	}

	if inf.OpenSearch == nil {
		return nil, apierror.Errorf("OpenSearch informer not supported in env: %q", env)
	}

	obj, err := inf.OpenSearch.Lister().ByNamespace(namespace).Get(name)
	if err != nil {
		return nil, fmt.Errorf("get OpenSearch: %w", err)
	}

	return toOpenSearch(obj.(*unstructured.Unstructured), env)
}

func (c client) getAccessForApplications(environmentName, openSearchName string, teamSlug slug.Slug) ([]*OpenSearchAccess, error) {
	infs, exists := c.informers[environmentName]
	if !exists {
		return nil, fmt.Errorf("unknown environment: %q", environmentName)
	}

	if infs.OpenSearch == nil {
		return nil, apierror.Errorf("OpenSearch informer not supported in environment: %q", environmentName)
	}

	access := make([]*OpenSearchAccess, 0)
	apps, err := infs.App.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("unable to list applications for team")
	}

	for _, a := range apps {
		app := &nais_io_v1alpha1.Application{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(a.(*unstructured.Unstructured).Object, app); err != nil {
			return nil, fmt.Errorf("converting to application: %w", err)
		}

		if app.Spec.OpenSearch == nil {
			continue
		}

		expectedName := "opensearch-" + string(teamSlug) + "-" + app.Spec.OpenSearch.Instance
		if app.Spec.OpenSearch != nil && expectedName == openSearchName {
			access = append(access, &OpenSearchAccess{
				Access:          app.Spec.OpenSearch.Access,
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

	return access, nil
}

func (c client) getAccessForJobs(environmentName, openSearchName string, teamSlug slug.Slug) ([]*OpenSearchAccess, error) {
	infs, exists := c.informers[environmentName]
	if !exists {
		return nil, fmt.Errorf("unknown environment: %q", environmentName)
	}

	if infs.OpenSearch == nil {
		return nil, apierror.Errorf("OpenSearch informer not supported in environment: %q", environmentName)
	}

	access := make([]*OpenSearchAccess, 0)
	naisJobs, err := infs.Naisjob.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("unable to list jobs for team")
	}

	for _, j := range naisJobs {
		job := &nais_io_v1.Naisjob{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(j.(*unstructured.Unstructured).Object, job); err != nil {
			return nil, fmt.Errorf("converting to job: %w", err)
		}

		if job.Spec.OpenSearch == nil {
			continue
		}

		expected := "opensearch-" + string(teamSlug) + "-" + job.Spec.OpenSearch.Instance
		if job.Spec.OpenSearch != nil && expected == openSearchName {
			access = append(access, &OpenSearchAccess{
				Access:          job.Spec.OpenSearch.Access,
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

	return access, nil
}

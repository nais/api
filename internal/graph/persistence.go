package graph

import (
	"fmt"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
)

func getRedisAccess(appInformer, jobInformer informers.GenericInformer, redisInstance string, teamSlug slug.Slug) (*model.Access, error) {
	access := &model.Access{}

	apps, err := appInformer.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("unable to list team apps")
	}

	for _, a := range apps {
		app := &nais_io_v1alpha1.Application{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(a.(*unstructured.Unstructured).Object, app); err != nil {
			return nil, fmt.Errorf("converting to application: %w", err)
		}

		for _, r := range app.Spec.Redis {
			if "redis-"+string(teamSlug)+"-"+r.Instance == redisInstance {
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

	naisJobs, err := jobInformer.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("unable to list team jobs")
	}

	for _, j := range naisJobs {
		job := &nais_io_v1.Naisjob{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(j.(*unstructured.Unstructured).Object, job); err != nil {
			return nil, fmt.Errorf("converting to job: %w", err)
		}

		for _, r := range job.Spec.Redis {
			if "redis-"+string(teamSlug)+"-"+r.Instance == redisInstance {
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

func getOpenSearchAccess(appInformer, jobInformer informers.GenericInformer, openSearchInstance string, teamSlug slug.Slug) (*model.Access, error) {
	access := &model.Access{}

	apps, err := appInformer.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("unable to list team apps")
	}

	for _, a := range apps {
		app := &nais_io_v1alpha1.Application{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(a.(*unstructured.Unstructured).Object, app); err != nil {
			return nil, fmt.Errorf("converting to application: %w", err)
		}

		if app.Spec.OpenSearch != nil &&
			"opensearch-"+string(teamSlug)+"-"+app.Spec.OpenSearch.Instance == openSearchInstance {
			access.Workloads = append(access.Workloads, model.AccessEntry{
				OwnerReference: &v1.OwnerReference{
					APIVersion: app.APIVersion,
					Kind:       app.Kind,
					Name:       app.Name,
					UID:        app.UID,
				},
				Role: app.Spec.OpenSearch.Access,
			})
		}
	}

	naisJobs, err := jobInformer.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("unable to list team jobs")
	}

	for _, j := range naisJobs {
		job := &nais_io_v1.Naisjob{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(j.(*unstructured.Unstructured).Object, job); err != nil {
			return nil, fmt.Errorf("converting to job: %w", err)
		}

		if job.Spec.OpenSearch != nil &&
			"opensearch-"+string(teamSlug)+"-"+job.Spec.OpenSearch.Instance == openSearchInstance {
			access.Workloads = append(access.Workloads, model.AccessEntry{
				OwnerReference: &v1.OwnerReference{
					APIVersion: job.APIVersion,
					Kind:       job.Kind,
					Name:       job.Name,
					UID:        job.UID,
				},
				Role: job.Spec.OpenSearch.Access,
			})
		}
	}

	return access, nil
}

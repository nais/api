package opensearch

import (
	"context"
	"fmt"
	"strconv"

	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/slug"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

type Client struct {
	informers k8s.ClusterInformers
	log       logrus.FieldLogger
	metrics   Metrics
}

func NewClient(informers k8s.ClusterInformers, log logrus.FieldLogger, costRepo database.CostRepo) *Client {
	return &Client{
		informers: informers,
		log:       log,
		metrics:   Metrics{log: log, costRepo: costRepo},
	}
}

func getAccess(apps []runtime.Object, naisJobs []runtime.Object, openSearchInstance string) (*model.Access, error) {
	access := &model.Access{}

	for _, a := range apps {
		app := &nais_io_v1alpha1.Application{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(a.(*unstructured.Unstructured).Object, app); err != nil {
			return nil, fmt.Errorf("converting to application: %w", err)
		}

		if app.Spec.OpenSearch != nil && app.Spec.OpenSearch.Instance == openSearchInstance {
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

	for _, j := range naisJobs {
		job := &nais_io_v1.Naisjob{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(j.(*unstructured.Unstructured).Object, job); err != nil {
			return nil, fmt.Errorf("converting to job: %w", err)
		}

		if job.Spec.OpenSearch != nil && job.Spec.OpenSearch.Instance == openSearchInstance {
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

func (c *Client) OpenSearchInstance(ctx context.Context, env string, teamSlug slug.Slug, openSearchName string) (*model.OpenSearch, error) {
	inf, exists := c.informers[env]
	if !exists {
		return nil, fmt.Errorf("unknown env: %q", env)
	}

	if inf.OpenSearch == nil {
		return nil, apierror.Errorf("openSearch informer not supported in env: %q", env)
	}

	obj, err := inf.OpenSearch.Lister().ByNamespace(string(teamSlug)).Get(openSearchName)
	if err != nil {
		return nil, fmt.Errorf("get openSearch: %w", err)
	}

	apps, err := inf.App.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
	if err != nil {
		c.log.WithError(err).Errorf("listing team apps")
	}
	naisJobs, err := inf.Naisjob.Lister().ByNamespace(string(teamSlug)).List(labels.Everything())
	if err != nil {
		c.log.WithError(err).Errorf("listing team jobs")
	}

	access, err := getAccess(apps, naisJobs, openSearchName)
	if err != nil {
		return nil, fmt.Errorf("getting access for openSearch instance: %w", err)
	}

	ret, err := model.ToOpenSearch(obj.(*unstructured.Unstructured), access, env)
	if err != nil {
		return nil, err
	}

	if ret.GQLVars.OwnerReference != nil {
		cost := c.metrics.CostForOpenSearchInstance(ctx, env, teamSlug, ret.GQLVars.OwnerReference.Name)
		ret.Cost = strconv.FormatFloat(cost, 'f', -1, 64)
	}

	return ret, nil
}

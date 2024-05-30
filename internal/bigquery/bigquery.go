package bigquery

import (
	"fmt"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/slug"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Client struct {
	informers k8s.ClusterInformers
	log       logrus.FieldLogger
}

func (c *Client) BigQueryDataset(env string, slug slug.Slug, name string) (*model.BigQueryDataset, error) {
	inf, exists := c.informers[env]
	if !exists {
		return nil, fmt.Errorf("unknown env: %q", env)
	}

	if inf.BigQuery == nil {
		return nil, apierror.Errorf("bigQueryDataset informer not supported in env: %q", env)
	}

	obj, err := inf.BigQuery.Lister().ByNamespace(string(slug)).Get(name)
	if err != nil {
		return nil, fmt.Errorf("get bigQueryDataset: %w", err)
	}

	return model.ToBigQueryDataset(obj.(*unstructured.Unstructured), env)
}

func NewClient(informers k8s.ClusterInformers, log logrus.FieldLogger) *Client {
	return &Client{
		informers: informers,
		log:       log,
	}
}

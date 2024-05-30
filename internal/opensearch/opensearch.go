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
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

	ret, err := model.ToOpenSearch(obj.(*unstructured.Unstructured), env)
	if err != nil {
		return nil, err
	}

	if ret.GQLVars.OwnerReference != nil {
		cost := c.metrics.CostForOpenSearchInstance(ctx, env, teamSlug, ret.GQLVars.OwnerReference.Name)
		ret.Cost = strconv.FormatFloat(cost, 'f', -1, 64)
	}

	return ret, nil
}

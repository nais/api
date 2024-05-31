package opensearch

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
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
	db        openSearchClientDatabase
}

type openSearchClientDatabase interface {
	database.CostRepo
	database.TeamRepo
}

func NewClient(informers k8s.ClusterInformers, log logrus.FieldLogger, db openSearchClientDatabase) *Client {
	return &Client{
		informers: informers,
		log:       log,
		db:        db,
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

	return ret, nil
}

func (c *Client) CostForOpenSearchInstance(ctx context.Context, env string, teamSlug slug.Slug, ownerName string) float64 {
	cost := 0.0
	now := time.Now()
	var from, to pgtype.Date
	_ = to.Scan(now)
	_ = from.Scan(now.AddDate(0, 0, -30))

	if sum, err := c.db.CostForInstance(ctx, "OpenSearch", from, to, teamSlug, ownerName, env); err != nil {
		c.log.WithError(err).Errorf("fetching cost")
	} else {
		cost = float64(sum)
	}
	return cost
}

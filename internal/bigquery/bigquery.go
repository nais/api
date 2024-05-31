package bigquery

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
	db        clientDatabase
}

type clientDatabase interface {
	database.CostRepo
	database.TeamRepo
}

func NewClient(informers k8s.ClusterInformers, log logrus.FieldLogger, db clientDatabase) *Client {
	return &Client{
		informers: informers,
		log:       log,
		db:        db,
	}
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

func (c *Client) CostForBiqQueryDataset(ctx context.Context, env string, teamSlug slug.Slug, ownerName string) float64 {
	cost := 0.0

	now := time.Now()
	var from, to pgtype.Date
	_ = to.Scan(now)
	_ = from.Scan(now.AddDate(0, 0, -30))

	if sum, err := c.db.CostForInstance(ctx, "BigQuery", from, to, teamSlug, ownerName, env); err != nil {
		c.log.WithError(err).Errorf("fetching cost")
	} else {
		cost = float64(sum)
	}

	return cost
}

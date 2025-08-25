package alerts

import (
	"context"

	"github.com/nais/api/internal/slug"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

type PrometheusAlertsClient interface {
	Rules(ctx context.Context, environment string, teamSlug slug.Slug) (promv1.RulesResult, error)
	RulesAll(ctx context.Context, teamSlug slug.Slug) (map[string]promv1.RulesResult, error)
}

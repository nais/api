package alerts

import (
	"context"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

type AlertsClient interface {
	Alerts(ctx context.Context, environment, team string) (promv1.AlertsResult, error)
	AlertsAll(ctx context.Context, team string) (map[string]promv1.AlertsResult, error)

	Rules(ctx context.Context, environment, team string) (promv1.RulesResult, error)
	RulesAll(ctx context.Context, team string) (map[string]promv1.RulesResult, error)
}

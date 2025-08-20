package alerts

import (
	"context"

	"github.com/nais/api/internal/slug"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

func ListGrafanaAlerts(ctx context.Context, environmentName string, teamSlug slug.Slug) ([]Alert, error) {
	retVal := make([]Alert, 0)
	c := fromContext(ctx).client

	r, err := c.Rules(ctx, environmentName, teamSlug.String())
	if err != nil {
		return nil, err
	}
	for _, rg := range r.Groups {
		for _, anyRule := range rg.Rules {
			switch ar := anyRule.(type) {
			case promv1.AlertingRule:
				retVal = append(retVal, &PrometheusAlert{Name: ar.Name})
			case promv1.RecordingRule:
				continue
			default:
				continue
			}
		}
	}
	return retVal, nil
}

package alerts

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

func ListPrometheusRules(ctx context.Context, environmentName string, teamSlug slug.Slug) ([]PrometheusAlert, error) {
	retVal := make([]PrometheusAlert, 0)
	c := fromContext(ctx).client

	r, err := c.Rules(ctx, environmentName, teamSlug)
	if err != nil {
		return nil, err
	}
	for _, rg := range r.Groups {
		for _, anyRule := range rg.Rules {
			switch ar := anyRule.(type) {
			case promv1.AlertingRule:
				var labels []*AlertKeyValue
				for k, v := range ar.Labels {
					labels = append(labels, &AlertKeyValue{
						Key:   string(k),
						Value: string(v),
					})
				}

				var annotations []*AlertKeyValue
				for k, v := range ar.Annotations {
					annotations = append(annotations, &AlertKeyValue{
						Key:   string(k),
						Value: string(v),
					})
				}
				retVal = append(retVal, PrometheusAlert{
					BaseAlert: BaseAlert{
						Name:            ar.Name,
						EnvironmentName: environmentName,
						TeamSlug:        teamSlug,
						State:           AlertState(strings.ToUpper(ar.State)),
						Labels:          labels,
						Query:           ar.Query,
						Annotations:     annotations,
						Duration:        ar.Duration,
					},
					RuleGroup: rg.Name,
				},
				)
			case promv1.RecordingRule:
				continue
			default:
				continue
			}
		}
	}
	return retVal, nil
}

func GetByIdent(ctx context.Context, id ident.Ident) (Alert, error) {
	team, env, alertName, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, team, env, alertName)
}

func Get(ctx context.Context, teamSlug slug.Slug, environmentName, ruleName string) (Alert, error) {
	c := fromContext(ctx).client

	r, err := c.Rules(ctx, environmentName, teamSlug)
	if err != nil {
		return nil, err
	}

	for _, rg := range r.Groups {
		for _, anyRule := range rg.Rules {
			switch ar := anyRule.(type) {
			case promv1.AlertingRule:
				if ar.Name == ruleName {
					var labels []*AlertKeyValue
					for k, v := range ar.Labels {
						labels = append(labels, &AlertKeyValue{
							Key:   string(k),
							Value: string(v),
						})
					}

					var annotations []*AlertKeyValue
					for k, v := range ar.Annotations {
						annotations = append(annotations, &AlertKeyValue{
							Key:   string(k),
							Value: string(v),
						})
					}
					return PrometheusAlert{
						BaseAlert: BaseAlert{
							Name:            ar.Name,
							EnvironmentName: environmentName,
							TeamSlug:        teamSlug,
							State:           AlertState(strings.ToUpper(ar.State)),
							Labels:          labels,
							Query:           ar.Query,
							Annotations:     annotations,
							Duration:        ar.Duration,
						},
						RuleGroup: rg.Name,
					}, nil
				}
			case promv1.RecordingRule:
				continue
			default:
				continue
			}
		}
	}
	return nil, nil
}

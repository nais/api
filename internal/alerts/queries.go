package alerts

import (
	"context"
	"fmt"
	"strings"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

func ListPrometheusRulesForTeam(ctx context.Context, teamSlug slug.Slug) ([]PrometheusAlert, error) {
	retVal := make([]PrometheusAlert, 0)
	c := fromContext(ctx).client

	r, err := c.RulesAll(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	for env, rules := range r {
		for _, rg := range rules.Groups {
			for _, anyRule := range rg.Rules {
				switch ar := anyRule.(type) {
				case promv1.AlertingRule:
					retVal = append(retVal, buildPromAlert(&ar, env, teamSlug, rg.Name))
				case *promv1.AlertingRule:
					retVal = append(retVal, buildPromAlert(ar, env, teamSlug, rg.Name))
				case promv1.RecordingRule, *promv1.RecordingRule:
					continue
				default:
					continue
				}
			}
		}
	}
	return retVal, nil
}

func ListPrometheusRulesForTeamInEnvironment(ctx context.Context, environmentName string, teamSlug slug.Slug) ([]PrometheusAlert, error) {
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
				retVal = append(retVal, buildPromAlert(&ar, environmentName, teamSlug, rg.Name))
			case *promv1.AlertingRule:
				retVal = append(retVal, buildPromAlert(ar, environmentName, teamSlug, rg.Name))
			case promv1.RecordingRule, *promv1.RecordingRule:
				continue
			default:
				continue
			}
		}
	}
	return retVal, nil
}

func buildPromAlert(ar *promv1.AlertingRule, env string, team slug.Slug, group string) PrometheusAlert {
	details := extractDetails(ar)

	return PrometheusAlert{
		BaseAlert: BaseAlert{
			Name:            ar.Name,
			EnvironmentName: env,
			TeamSlug:        team,
			State:           AlertState(strings.ToUpper(ar.State)),
			Query:           ar.Query,
			Duration:        ar.Duration,
		},
		RuleGroup: group,
		Details:   details,
	}
}

func extractDetails(ar *promv1.AlertingRule) []*PrometheusAlertDetails {
	details := make([]*PrometheusAlertDetails, 0, len(ar.Alerts))
	for _, a := range ar.Alerts {
		get := func(key model.LabelName) string {
			if v := a.Annotations[key]; v != "" {
				return string(v)
			}
			return string(ar.Annotations[key])
		}

		details = append(details, &PrometheusAlertDetails{
			Action:      get(model.LabelName("action")),
			Consequence: get(model.LabelName("consequence")),
			Summary:     get(model.LabelName("summary")),
			Since:       a.ActiveAt,
		})
	}
	return details
}

func GetByIdent(ctx context.Context, id ident.Ident) (Alert, error) {
	alertType, team, env, alertName, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, alertType, team, env, alertName)
}

func Get(ctx context.Context, alertType AlertType, teamSlug slug.Slug, environmentName, ruleName string) (Alert, error) {
	if alertType != AlertTypePrometheus {
		return nil, fmt.Errorf("unsupported alert type: %s", alertType)
	}
	return getPrometheusRule(ctx, environmentName, teamSlug, ruleName)
}

func getPrometheusRule(ctx context.Context, environmentName string, teamSlug slug.Slug, ruleName string) (Alert, error) {
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
					return buildPromAlert(&ar, environmentName, teamSlug, rg.Name), nil
				}
			case *promv1.AlertingRule:
				if ar.Name == ruleName {
					return buildPromAlert(ar, environmentName, teamSlug, rg.Name), nil
				}
			}
		}
	}
	return nil, fmt.Errorf("alert not found: %s", ruleName)
}

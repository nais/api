package application

import (
	"context"
	"fmt"
	"net/url"
	"regexp"

	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/thirdparty/promclient"
	prom "github.com/prometheus/common/model"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

type IngressMetricsClient interface {
	Query(ctx context.Context, environment string, query string, opts ...promclient.QueryOption) (prom.Vector, error)
}

const (
	ingressRequests = `sum(rate(nginx_ingress_controller_requests{host=%q, path=%q}[2m]))`
	errorRate       = `sum(rate(nginx_ingress_controller_requests{status!~"^[23].*", host=%q, path=%q}[2m]))`
)

func ensuredVal(v prom.Vector) float64 {
	if len(v) == 0 {
		return 0
	}
	return float64(v[0].Value)
}

func labelSelectorFor(team, app string) (labels.Selector, error) {
	teamReq, err := labels.NewRequirement("team", selection.Equals, []string{team})
	if err != nil {
		return nil, fmt.Errorf("failed to create team label requirement: %w", err)
	}
	appReq, err := labels.NewRequirement("app", selection.Equals, []string{app})
	if err != nil {
		return nil, fmt.Errorf("failed to create app label requirement: %w", err)
	}

	return labels.NewSelector().Add(*teamReq, *appReq), nil
}

// ingressMetric finds the best-matching ingress path (longest regex match) and queries Prometheus using it.
func ingressMetric(ctx context.Context, obj *IngressMetrics, promQueryFmt string) (float64, error) {
	c := fromContext(ctx).client

	selector, err := labelSelectorFor(obj.TeamSlug.String(), obj.ApplicationName)
	if err != nil {
		return 0, err
	}

	ingressURL, err := url.Parse(obj.URL)
	if err != nil {
		return 0, fmt.Errorf("failed to parse ingress URL %q: %w", obj.URL, err)
	}

	urlPath := ingressURL.EscapedPath()
	if urlPath == "" {
		urlPath = "/"
	}

	ingresses := fromContext(ctx).ingressWatcher.GetByCluster(obj.EnvironmentName, watcher.WithLabels(selector))

	var bestMatch string
	var bestHost string
	maxLength := -1

	for _, ing := range ingresses {
		for _, rule := range ing.Obj.Spec.Rules {
			if rule.Host != ingressURL.Hostname() || rule.HTTP == nil {
				continue
			}
			for _, p := range rule.HTTP.Paths {
				re, err := regexp.Compile("^" + p.Path + "$")
				if err != nil {
					continue // skip invalid regex
				}
				if re.MatchString(urlPath) {
					if len(p.Path) > maxLength {
						bestMatch = p.Path
						bestHost = rule.Host
						maxLength = len(p.Path)
					}
				}
			}
		}
	}

	if bestMatch != "" {
		query := fmt.Sprintf(promQueryFmt, bestHost, bestMatch)
		v, err := c.Query(ctx, obj.EnvironmentName, query)
		if err != nil {
			return 0, err
		}
		return ensuredVal(v), nil
	}

	return 0, nil
}

func RequestsPerSecondForIngress(ctx context.Context, obj *IngressMetrics) (float64, error) {
	return ingressMetric(ctx, obj, ingressRequests)
}

func ErrorsPerSecondForIngress(ctx context.Context, obj *IngressMetrics) (float64, error) {
	return ingressMetric(ctx, obj, errorRate)
}

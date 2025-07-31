package application

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/thirdparty/promclient"
	prom "github.com/prometheus/common/model"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

type IngressMetricsClient interface {
	Query(ctx context.Context, environment string, query string, opts ...promclient.QueryOption) (prom.Vector, error)
	// QueryAll(ctx context.Context, query string, opts ...promclient.QueryOption) (map[string]prom.Vector, error)
	// QueryRange(ctx context.Context, environment string, query string, promRange promv1.Range) (prom.Value, promv1.Warnings, error)
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

func RequestsPerSecondForIngress(ctx context.Context, obj *IngressMetrics) (float64, error) {
	q := ingressRequests
	c := fromContext(ctx).client

	teamRequirement, err := labels.NewRequirement("team", selection.Equals, []string{obj.TeamSlug.String()})
	if err != nil {
		return 0, fmt.Errorf("failed to create label requirement: %w", err)
	}
	appRequirement, err := labels.NewRequirement("app", selection.Equals, []string{obj.ApplicationName})
	if err != nil {
		return 0, fmt.Errorf("failed to create label requirement: %w", err)
	}
	
	selector := labels.NewSelector()
	selector = selector.Add(*teamRequirement, *appRequirement)
	
	ingressURL, err := url.Parse(obj.URL)
	if err != nil {
		return 0, fmt.Errorf("failed to parse ingress URL %q: %w", obj.URL, err)
	}

	ingress := fromContext(ctx).ingressWatcher.GetByCluster(obj.EnvironmentName, watcher.WithLabels(selector))
	for _, ingress := range ingress {
		for _, rules := range ingress.Obj.Spec.Rules {
			if rules.Host == ingressURL.Hostname() {
				for _, path := range rules.HTTP.Paths {
					if strings.HasPrefix(path.Path, ingressURL.EscapedPath()) {
						v, err := c.Query(ctx, obj.EnvironmentName, fmt.Sprintf(q, rules.Host, path.Path))
						if err != nil {
							return 0, err
						}
						return ensuredVal(v), nil
					}
				}
			}
		}
	}
	return 0, nil
}

func ErrorsPerSecondForIngress(ctx context.Context, obj *IngressMetrics) (float64, error) {
	q := errorRate
	c := fromContext(ctx).client

	teamRequirement, err := labels.NewRequirement("team", selection.Equals, []string{obj.TeamSlug.String()})
	if err != nil {
		return 0, fmt.Errorf("failed to create label requirement: %w", err)
	}
	appRequirement, err := labels.NewRequirement("app", selection.Equals, []string{obj.ApplicationName})
	if err != nil {
		return 0, fmt.Errorf("failed to create label requirement: %w", err)
	}
	
	selector := labels.NewSelector()
	selector = selector.Add(*teamRequirement, *appRequirement)
	
	ingressURL, err := url.Parse(obj.URL)
	if err != nil {
		return 0, fmt.Errorf("failed to parse ingress URL %q: %w", obj.URL, err)
	}

	ingress := fromContext(ctx).ingressWatcher.GetByCluster(obj.EnvironmentName, watcher.WithLabels(selector))
	for _, ingress := range ingress {
		for _, rules := range ingress.Obj.Spec.Rules {
			if rules.Host == ingressURL.Hostname() {
				for _, path := range rules.HTTP.Paths {
					if strings.HasPrefix(path.Path, ingressURL.EscapedPath()) {
						v, err := c.Query(ctx, obj.EnvironmentName, fmt.Sprintf(q, rules.Host, path.Path))
						if err != nil {
							return 0, err
						}
						return ensuredVal(v), nil
					}
				}
			}
		}
	}
	return 0, nil
}

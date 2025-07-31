package application

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/nais/api/internal/thirdparty/promclient"
	prom "github.com/prometheus/common/model"
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

// ingressMetric finds the best-matching ingress path (longest regex match) and queries Prometheus using it.
func ingressMetric(ctx context.Context, obj *IngressMetrics, promQueryFmt string) (float64, error) {
	c := fromContext(ctx).client

	ingressURL, err := url.Parse(obj.Ingress.URL)
	if err != nil {
		return 0, fmt.Errorf("failed to parse ingress URL %q: %w", obj.Ingress.URL, err)
	}

	if len(ingressURL.Path) > 1 {
		ingressURL.Path = strings.TrimRight(ingressURL.Path, "/") + "(/.*)?"
	} else {
		ingressURL.Path = "/"
	}

	query := fmt.Sprintf(promQueryFmt, ingressURL.Host, ingressURL.Path)
	a, err := c.Query(ctx, obj.Ingress.EnvironmentName, query)
	if err != nil {
		return 0, fmt.Errorf("failed to query Prometheus for ingress %q: %w", obj.Ingress.URL, err)
	}
	return ensuredVal(a), nil
}

func RequestsPerSecondForIngress(ctx context.Context, obj *IngressMetrics) (float64, error) {
	return ingressMetric(ctx, obj, ingressRequests)
}

func ErrorsPerSecondForIngress(ctx context.Context, obj *IngressMetrics) (float64, error) {
	return ingressMetric(ctx, obj, errorRate)
}

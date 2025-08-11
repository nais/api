package application

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/thirdparty/promclient"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prom "github.com/prometheus/common/model"
)

type IngressMetricsClient interface {
	Query(ctx context.Context, environment string, query string, opts ...promclient.QueryOption) (prom.Vector, error)
	QueryRange(ctx context.Context, environment string, query string, promRange promv1.Range) (prom.Value, promv1.Warnings, error)
}

const (
	ingressRequests = `sum(rate(nginx_ingress_controller_requests{host=%q, path=%q}[2m]))`
	errorRate       = `sum(rate(nginx_ingress_controller_requests{status!~"^[23].*", host=%q, path=%q}[2m]))`

	errorsSeries   = `sum(rate(nginx_ingress_controller_requests{service=%q, host=%q, status=~"[4-5].*"}[2m])) by (host)`
	requestsSeries = `sum(rate(nginx_ingress_controller_requests{service=%q, host=%q}[2m])) by (host)`
)

func ensuredVal(v prom.Vector) float64 {
	if len(v) == 0 {
		return 0
	}
	return float64(v[0].Value)
}

func SeriesForIngress(ctx context.Context, obj *IngressMetrics, input IngressMetricsInput) ([]*IngressMetricSample, error) {
	url := strings.Split(obj.Ingress.URL, "https://")[1]
	query := fmt.Sprintf(errorsSeries, obj.Ingress.ApplicationName, url)
	if input.Type == IngressMetricsTypeRequestsPerSecond {
		query = fmt.Sprintf(requestsSeries, obj.Ingress.ApplicationName, url)
	}

	c := fromContext(ctx).client

	v, warnings, err := c.QueryRange(ctx, environmentmapper.ClusterName(obj.Ingress.EnvironmentName), query, promv1.Range{Start: input.Start, End: input.End, Step: time.Duration(input.Step()) * time.Second})
	if err != nil {
		return nil, err
	}
	if len(warnings) > 0 {
		return nil, fmt.Errorf("prometheus query warnings: %s", strings.Join(warnings, ", "))
	}

	matrix, ok := v.(prom.Matrix)
	if !ok {
		return nil, fmt.Errorf("expected prometheus matrix, got %T", v)
	}

	ret := make([]*IngressMetricSample, 0)
	for _, sample := range matrix {
		for _, value := range sample.Values {
			ret = append(ret, &IngressMetricSample{
				Value:     float64(value.Value),
				Timestamp: value.Timestamp.Time(),
				Instance:  "https://" + string(sample.Metric["host"]),
			})
		}
	}

	return ret, nil
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
	a, err := c.Query(ctx, environmentmapper.ClusterName(obj.Ingress.EnvironmentName), query)
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

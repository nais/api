package metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/nais/api/internal/thirdparty/promclient"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prom "github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"k8s.io/utils/ptr"
)

type ctxKey int

const loadersKey ctxKey = iota

type loaders struct {
	client promclient.QueryClient
	log    logrus.FieldLogger
}

// NewLoaderContext creates a new context with the metrics loaders
func NewLoaderContext(ctx context.Context, client promclient.QueryClient, log logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loadersKey, &loaders{
		client: client,
		log:    log,
	})
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

// Query executes a Prometheus query based on the input
func Query(ctx context.Context, input MetricsQueryInput) (*MetricsQueryResult, error) {
	loader := fromContext(ctx)

	// Determine query time
	queryTime := ptr.Deref(input.Time, time.Now().Add(-5*time.Minute))

	// Execute range query if range is specified
	if input.Range != nil {
		return executeRangeQuery(ctx, loader, input)
	}

	// Execute instant query
	return executeInstantQuery(ctx, loader, input, input.EnvironmentName, queryTime)
}

func executeInstantQuery(ctx context.Context, loader *loaders, input MetricsQueryInput, environmentName string, queryTime time.Time) (*MetricsQueryResult, error) {
	vector, err := loader.client.Query(ctx, environmentName, input.Query, promclient.WithTime(queryTime))
	if err != nil {
		return nil, fmt.Errorf("failed to query prometheus: %w", err)
	}

	series := convertVectorToSeries(vector)
	return &MetricsQueryResult{
		Series:   series,
		Warnings: nil,
	}, nil
}

func executeRangeQuery(ctx context.Context, loader *loaders, input MetricsQueryInput) (*MetricsQueryResult, error) {
	promRange := promv1.Range{
		Start: input.Range.Start,
		End:   input.Range.End,
		Step:  time.Duration(input.Range.Step) * time.Second,
	}

	value, warnings, err := loader.client.QueryRange(ctx, input.EnvironmentName, input.Query, promRange)
	if err != nil {
		return nil, fmt.Errorf("failed to execute range query: %w", err)
	}

	if len(warnings) > 0 {
		loader.log.WithFields(logrus.Fields{
			"environment": input.EnvironmentName,
			"warnings":    warnings,
		}).Warn("prometheus query warnings")
	}

	matrix, ok := value.(prom.Matrix)
	if !ok {
		return nil, fmt.Errorf("expected prometheus matrix, got %T", value)
	}

	series := convertMatrixToSeries(matrix)

	return &MetricsQueryResult{
		Series:   series,
		Warnings: warnings,
	}, nil
}

// convertVectorToSeries converts a Prometheus vector to MetricSeries
func convertVectorToSeries(vector prom.Vector) []*MetricSeries {
	series := make([]*MetricSeries, 0, len(vector))

	for _, sample := range vector {
		labels := make([]*MetricLabel, 0, len(sample.Metric))
		for name, value := range sample.Metric {
			labels = append(labels, &MetricLabel{
				Name:  string(name),
				Value: string(value),
			})
		}

		series = append(series, &MetricSeries{
			Labels: labels,
			Values: []*MetricValue{
				{
					Timestamp: sample.Timestamp.Time(),
					Value:     float64(sample.Value),
				},
			},
		})
	}

	return series
}

// convertMatrixToSeries converts a Prometheus matrix to MetricSeries
func convertMatrixToSeries(matrix prom.Matrix) []*MetricSeries {
	series := make([]*MetricSeries, 0, len(matrix))

	for _, sampleStream := range matrix {
		labels := make([]*MetricLabel, 0, len(sampleStream.Metric))
		for name, value := range sampleStream.Metric {
			labels = append(labels, &MetricLabel{
				Name:  string(name),
				Value: string(value),
			})
		}

		values := make([]*MetricValue, 0, len(sampleStream.Values))
		for _, pair := range sampleStream.Values {
			values = append(values, &MetricValue{
				Timestamp: pair.Timestamp.Time(),
				Value:     float64(pair.Value),
			})
		}

		series = append(series, &MetricSeries{
			Labels: labels,
			Values: values,
		})
	}

	return series
}

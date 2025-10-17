package metrics

import (
	"time"
)

// MetricLabel represents a key-value pair for a Prometheus label.
type MetricLabel struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// MetricValue represents a single data point in a time series.
type MetricValue struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// MetricSeries represents a time series with its labels and data points.
type MetricSeries struct {
	Labels []*MetricLabel `json:"labels"`
	Values []*MetricValue `json:"values"`
}

// MetricsQueryInput represents the input for querying Prometheus metrics.
type MetricsQueryInput struct {
	Query           string             `json:"query"`
	EnvironmentName string             `json:"environmentName"`
	Time            *time.Time         `json:"time,omitempty"`
	Range           *MetricsRangeInput `json:"range,omitempty"`
}

// MetricsRangeInput represents the input for Prometheus range queries.
type MetricsRangeInput struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Step  int       `json:"step"`
}

// MetricsQueryResult represents the result from a Prometheus metrics query.
type MetricsQueryResult struct {
	Series   []*MetricSeries `json:"series"`
	Warnings []string        `json:"warnings,omitempty"`
}

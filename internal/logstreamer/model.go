package logstreamer

import (
	"time"

	"github.com/nais/api/internal/slug"
)

type LogLine struct {
	Time    time.Time       `json:"time"`
	Message string          `json:"message"`
	Labels  []*LogLineLabel `json:"labels"`
}

type LogLineLabel struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type queryOptions struct {
	limit     int
	start     time.Time
	end       time.Time
	direction string
}

type LogSubscriptionFilter struct {
	TeamSlug        slug.Slug      `json:"teamSlug"`
	EnvironmentName *string        `json:"environmentName"`
	WorkloadName    *string        `json:"workloadName"`
	Since           *time.Duration `json:"since"`
	Limit           *int           `json:"limit"`
	opts            queryOptions
}

func (f *LogSubscriptionFilter) Query() string {
	builder := NewQueryBuilder().AddNamespace(f.TeamSlug.String())

	if f.EnvironmentName != nil && *f.EnvironmentName != "" {
		builder.AddCluster(*f.EnvironmentName)
	}

	if f.WorkloadName != nil && *f.WorkloadName != "" {
		builder = builder.AddApp(*f.WorkloadName)
	}

	return builder.Build()
}

func (f *LogSubscriptionFilter) WithLimit(limit int) *LogSubscriptionFilter {
	f.opts.limit = limit
	return f
}

func (f *LogSubscriptionFilter) WithStart(start time.Time) *LogSubscriptionFilter {
	f.opts.start = start
	return f
}

func (f *LogSubscriptionFilter) WithEnd(end time.Time) *LogSubscriptionFilter {
	f.opts.end = end
	return f
}

func (f *LogSubscriptionFilter) WithDirection(direction string) *LogSubscriptionFilter {
	f.opts.direction = direction
	return f
}

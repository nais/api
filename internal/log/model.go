package log

import (
	"time"

	"github.com/nais/api/internal/slug"
)

type LogLine struct {
	Time    time.Time         `json:"time"`
	Message string            `json:"message"`
	Labels  map[string]string `json:"labels"`
}

type queryOptions struct {
	limit     int
	start     time.Time
	end       time.Time
	direction string
}

type LogSubscriptionFilter struct {
	Team        slug.Slug `json:"team"`
	Environment string    `json:"environment"`
	Application *string   `json:"application"`
	opts        queryOptions
}

func (f *LogSubscriptionFilter) Query() string {
	builder := NewQueryBuilder().AddNamespace(f.Team.String())

	if f.Environment != "" {
		builder.AddCluster(f.Environment)
	}

	if f.Application != nil {
		builder = builder.AddApp(*f.Application)
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

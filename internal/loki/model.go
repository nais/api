package loki

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/grafana/loki/v3/pkg/loghttp"
	"github.com/nais/api/internal/environment"
	"github.com/nais/api/internal/validate"
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

type LogSubscriptionInitialBatch struct {
	Start *time.Time `json:"start"`
	Limit int        `json:"limit"`
}

type LogSubscriptionFilter struct {
	EnvironmentName string                      `json:"environmentName"`
	Query           string                      `json:"query"`
	InitialBatch    LogSubscriptionInitialBatch `json:"initialBatch"`
}

func (f *LogSubscriptionFilter) Validate(ctx context.Context) error {
	if f.InitialBatch.Start == nil {
		f.InitialBatch.Start = new(time.Now().Add(-time.Hour))
	}

	verr := validate.New()

	if _, err := environment.Get(ctx, f.EnvironmentName); err != nil {
		verr.Add("environmentName", "Environment does not exist.")
	}

	if _, err := loghttp.ParseTailQuery(&http.Request{Form: f.lokiQueryParameters()}); err != nil {
		verr.Add("query", "Unable to parse log query.")
	}

	if f.InitialBatch.Start.After(time.Now()) {
		verr.Add("initialBatch.start", "Start time cannot be in the future.")
	}

	return verr.NilIfEmpty()
}

func (f *LogSubscriptionFilter) lokiQueryParameters() url.Values {
	values := url.Values{}

	values.Set("query", f.Query)
	values.Set("limit", fmt.Sprintf("%d", f.InitialBatch.Limit))
	values.Set("start", fmt.Sprintf("%d", f.InitialBatch.Start.UnixNano()))

	return values
}

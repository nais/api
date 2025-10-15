package loki

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/grafana/loki/v3/pkg/loghttp"
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
	Since time.Duration `json:"since"`
	Limit int           `json:"limit"`
}

type LogSubscriptionFilter struct {
	Query        string                      `json:"query"`
	InitialBatch LogSubscriptionInitialBatch `json:"initialBatch"`
}

func (f *LogSubscriptionFilter) Validate() error {
	verr := validate.New()

	if _, err := loghttp.ParseTailQuery(&http.Request{Form: f.lokiQueryParameters()}); err != nil {
		verr.Add("query", "Unable to parse query")
	}

	return verr.NilIfEmpty()
}

func (f *LogSubscriptionFilter) lokiQueryParameters() url.Values {
	values := url.Values{}

	values.Set("query", f.Query)
	values.Set("limit", fmt.Sprintf("%d", f.InitialBatch.Limit))
	values.Set("start", fmt.Sprintf("%d", time.Now().Add(-f.InitialBatch.Since).UnixNano()))

	return values
}

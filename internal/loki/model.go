package loki

import (
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

type LogSubscriptionFilter struct {
	Query string         `json:"query"`
	Since *time.Duration `json:"since"`
	Limit *int           `json:"limit"`
}

func (f *LogSubscriptionFilter) Validate() error {
	verr := validate.New()
	v := url.Values{}
	v.Set("query", f.Query)

	if _, err := loghttp.ParseTailQuery(&http.Request{Form: v}); err != nil {
		verr.Add("query", "Unable to parse query")
	}

	return verr.NilIfEmpty()
}

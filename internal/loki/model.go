package loki

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/grafana/loki/v3/pkg/loghttp"
	"github.com/grafana/loki/v3/pkg/logql/syntax"
	"github.com/nais/api/internal/environment"
	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/validate"
	"github.com/prometheus/prometheus/model/labels"
)

const clusterNameLabel = "k8s_cluster_name"

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

	values, err := f.lokiQueryParameters()
	if err != nil {
		verr.Add("query", "Unable to parse log query: %v.", err.Error())
	} else if _, err := loghttp.ParseTailQuery(&http.Request{Form: values}); err != nil {
		verr.Add("query", "Unable to parse log query: %v.", err.Error())
	}

	if f.InitialBatch.Start.After(time.Now()) {
		verr.Add("initialBatch.start", "Start time cannot be in the future.")
	}

	return verr.NilIfEmpty()
}

func (f *LogSubscriptionFilter) lokiQueryParameters() (url.Values, error) {
	values := url.Values{}

	q, err := injectEnvLabel(f.Query, f.EnvironmentName)
	if err != nil {
		return nil, err
	}
	values.Set("query", q)
	values.Set("limit", fmt.Sprintf("%d", f.InitialBatch.Limit))
	values.Set("start", fmt.Sprintf("%d", f.InitialBatch.Start.UnixNano()))

	return values, nil
}

func injectEnvLabel(query, clusterName string) (string, error) {
	expr, err := syntax.ParseExpr(query)
	if err != nil {
		return "", err
	}

	expr.Walk(func(e syntax.Expr) bool {
		matchers, ok := e.(*syntax.MatchersExpr)
		if !ok {
			return true
		}

		for _, m := range matchers.Matchers() {
			if m.Name == clusterNameLabel {
				return true
			}
		}

		matchers.AppendMatchers([]*labels.Matcher{{
			Type:  labels.MatchEqual,
			Name:  clusterNameLabel,
			Value: environmentmapper.ClusterName(clusterName),
		}})

		return true
	})

	return expr.String(), nil
}

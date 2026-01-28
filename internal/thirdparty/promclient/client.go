package promclient

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nais/api/internal/slug"
	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prom "github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
)

type QueryClient interface {
	Query(ctx context.Context, environment string, query string, opts ...QueryOption) (prom.Vector, error)
	QueryAll(ctx context.Context, query string, opts ...QueryOption) (prom.Vector, error)
	QueryRange(ctx context.Context, environment string, query string, promRange promv1.Range) (prom.Value, promv1.Warnings, error)
}

type RulesClient interface {
	Rules(ctx context.Context, environment string, teamSlug slug.Slug) (promv1.RulesResult, error)
	RulesAll(ctx context.Context, teamSlug slug.Slug) (map[string]promv1.RulesResult, error)
}

type Client interface {
	QueryClient
	RulesClient
}

type QueryOpts struct {
	Time time.Time
}

type QueryOption func(*QueryOpts)

func WithTime(t time.Time) QueryOption {
	return func(opts *QueryOpts) {
		opts.Time = t
	}
}

type RealClient struct {
	mimirMetrics promv1.API
	mimirRules   promv1.API
	log          logrus.FieldLogger
}

type mimirRoundTrip struct {
	HeaderValue string
}

func (r mimirRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("X-Scope-OrgID", r.HeaderValue)
	return http.DefaultTransport.RoundTrip(req)
}

func New(tenant string, log logrus.FieldLogger) (*RealClient, error) {
	mimirMetrics, err := api.NewClient(api.Config{Address: "http://mimir-query-frontend:8080/prometheus", RoundTripper: mimirRoundTrip{HeaderValue: "nais"}})
	if err != nil {
		return nil, err
	}

	mimirAlerts, err := api.NewClient(api.Config{Address: "http://mimir-ruler:8080/prometheus", RoundTripper: mimirRoundTrip{HeaderValue: "tenant"}})
	if err != nil {
		return nil, err
	}

	return &RealClient{
		mimirMetrics: promv1.NewAPI(mimirMetrics),
		mimirRules:   promv1.NewAPI(mimirAlerts),
		log:          log,
	}, nil
}

func (c *RealClient) QueryAll(ctx context.Context, query string, opts ...QueryOption) (prom.Vector, error) {
	return c.Query(ctx, "query-all", query, opts...)
}

func (c *RealClient) Query(ctx context.Context, environmentName string, query string, opts ...QueryOption) (prom.Vector, error) {
	client := c.mimirMetrics

	opt := &QueryOpts{
		Time: time.Now().Add(-5 * time.Minute),
	}
	for _, fn := range opts {
		fn(opt)
	}

	v, warnings, err := client.Query(ctx, query, opt.Time)
	if err != nil {
		return nil, err
	}

	if len(warnings) > 0 {
		c.log.WithFields(logrus.Fields{
			"environment": environmentName,
			"warnings":    strings.Join(warnings, ", "),
		}).Warn("prometheus query warnings")
	}

	vector, ok := v.(prom.Vector)
	if !ok {
		return nil, fmt.Errorf("expected prometheus vector, got %T", v)
	}

	return vector, nil
}

func (c *RealClient) QueryRange(ctx context.Context, environment string, query string, promRange promv1.Range) (prom.Value, promv1.Warnings, error) {
	client := c.mimirMetrics
	return client.QueryRange(ctx, query, promRange)
}

func (c *RealClient) Rules(ctx context.Context, environment string, teamSlug slug.Slug) (promv1.RulesResult, error) {
	res, err := c.mimirRules.Rules(ctx)
	if err != nil {
		return promv1.RulesResult{}, err
	}

	if teamSlug == "" && environment == "" {
		return res, nil
	}

	return filterRulesByTeam(res, environment, teamSlug), nil
}

func (c *RealClient) RulesAll(ctx context.Context, teamSlug slug.Slug) (map[string]promv1.RulesResult, error) {
	res, err := c.Rules(ctx, "", teamSlug)
	if err != nil {
		return nil, err
	}

	rules := map[string]promv1.RulesResult{}
	for _, group := range res.Groups {
		splittedFile := strings.Split(group.File, "/")
		if len(splittedFile) < 1 {
			continue
		}

		cluster := splittedFile[0]

		rule := rules[cluster]
		rule.Groups = append(rules[cluster].Groups, group)
		rules[cluster] = rule
	}

	return rules, nil
}

func filterRulesByTeam(in promv1.RulesResult, env string, teamSlug slug.Slug) promv1.RulesResult {
	out := promv1.RulesResult{}
	out.Groups = make([]promv1.RuleGroup, 0, len(in.Groups))

	for _, g := range in.Groups {
		splittedFile := strings.Split(g.File, "/")
		if len(splittedFile) < 2 {
			continue
		}

		cluster := splittedFile[0]
		namespace := splittedFile[1]

		var filtered promv1.Rules
		if namespace == teamSlug.String() {
			if env == "" || cluster == env {
				for _, r := range g.Rules {
					if ar, ok := r.(promv1.AlertingRule); ok {
						filtered = append(filtered, ar)
					}
				}
			}
		}

		if len(filtered) > 0 {
			out.Groups = append(out.Groups, promv1.RuleGroup{
				Name:     g.Name,
				File:     g.File,
				Rules:    filtered,
				Interval: g.Interval,
			})
		}
	}

	return out
}

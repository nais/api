package fake

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"

	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team"

	"github.com/nais/api/internal/thirdparty/promclient"
	"github.com/nais/api/internal/workload/application"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prom "github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	parser "github.com/prometheus/prometheus/promql/parser"
)

const teamLabelKey = "namespace"

type FakeClient struct {
	environments []string
	now          func() prom.Time

	mu     sync.Mutex
	random *rand.Rand
}

func NewFakeClient(random *rand.Rand, nowFunc func() prom.Time) *FakeClient {
	if nowFunc == nil {
		nowFunc = prom.Now
	}
	if random == nil {
		random = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	}
	return &FakeClient{
		environments: []string{"test", "dev"},
		random:       random,
		now:          nowFunc,
	}
}

func (c *FakeClient) QueryAll(ctx context.Context, query string, opts ...promclient.QueryOption) (prom.Vector, error) {
	var vectors prom.Vector
	for _, env := range c.environments {
		vec, err := c.Query(ctx, env, query, opts...)
		if err != nil {
			return vec, err
		}

		vectors = append(vec, vectors...)
	}

	return vectors, nil
}

func (c *FakeClient) Query(ctx context.Context, environment string, query string, opts ...promclient.QueryOption) (prom.Vector, error) {
	opt := promclient.QueryOpts{
		Time: c.now().Time(),
	}
	for _, o := range opts {
		o(&opt)
	}

	expr, err := parser.ParseExpr(query)
	if err != nil {
		return nil, err
	}

	var (
		teamSlug       slug.Slug
		workload       string
		unit           string
		labelsToCreate []string
	)

	switch expr := expr.(type) {
	case *parser.AggregateExpr:
		labelsToCreate = expr.Grouping
		teamSlug, workload, unit, err = c.selector(expr.Expr)

	case *parser.VectorSelector:
		for _, matcher := range expr.LabelMatchers {
			labelsToCreate = append(labelsToCreate, matcher.Name)
		}
		labelsToCreate = append(labelsToCreate, "k8s_cluster_name")
		teamSlug, workload, unit, err = c.selector(expr)

	case *parser.Call:
		vectorSelector, ok := expr.Args[0].(*parser.VectorSelector)
		if !ok {
			matrixSelector, ok := expr.Args[0].(*parser.MatrixSelector)
			if !ok {
				return nil, fmt.Errorf("query: unexpected argument type %T", expr.Args[0])
			}
			vectorSelector, ok = matrixSelector.VectorSelector.(*parser.VectorSelector)
			if !ok {
				return nil, fmt.Errorf("query: unexpected argument type %T", matrixSelector.VectorSelector)
			}
		}
		for _, matcher := range vectorSelector.LabelMatchers {
			labelsToCreate = append(labelsToCreate, matcher.Name)
		}
		labelsToCreate = append(labelsToCreate, []string{"pod", "k8s_cluster_name"}...)
		teamSlug, workload, unit, err = c.selector(expr)

	default:
		return nil, fmt.Errorf("query: unexpected expression type %T", expr)
	}
	if err != nil {
		return nil, err
	}

	makeLabels := func() prom.Metric {
		labels := prom.Metric{}
		for _, label := range labelsToCreate {
			switch label {
			case "namespace":
				labels["namespace"] = prom.LabelValue(teamSlug)
			case "workload_namespace":
				labels["workload_namespace"] = prom.LabelValue(teamSlug)
			case "container":
				labels["container"] = prom.LabelValue(workload)
			case "pod":
				labels["pod"] = prom.LabelValue(fmt.Sprintf("%s-%s", workload, "1"))
			}
		}
		// TODO: Støtte de spørringene som har et miljø satt, og QueryAll som svarer med flere miljøer
		labels["k8s_cluster_name"] = prom.LabelValue(environment)
		return labels
	}

	value := func() prom.SampleValue {
		c.mu.Lock()
		defer c.mu.Unlock()

		switch unit {
		case "core":
			return prom.SampleValue(c.random.Float64() * 2)
		case "byte":
			return prom.SampleValue(c.random.IntN(1024*1024*1024) + 16*1024*1024)
		default:
			return prom.SampleValue(c.random.IntN(100))
		}
	}

	ret := prom.Vector{}
	if teamSlug != "" {
		if workload == "" {
			for _, app := range application.ListAllForTeam(ctx, teamSlug, nil, nil) {
				if app.EnvironmentName != environment {
					continue
				}
				workload = app.Name
				ret = append(ret, &prom.Sample{
					Timestamp: prom.TimeFromUnix(opt.Time.Unix()),
					Metric:    makeLabels(),
					Value:     value(),
				})
			}
			return ret, nil
		}

		ret = append(ret, &prom.Sample{
			Timestamp: prom.TimeFromUnix(opt.Time.Unix()),
			Metric:    makeLabels(),
			Value:     value(),
		})
	} else {
		page, err := pagination.ParsePage(new(10000), nil, nil, nil)
		if err != nil {
			return nil, err
		}

		teams, err := team.List(ctx, page, nil, nil)
		if err != nil {
			return nil, err
		}

		for _, t := range teams.Nodes() {
			teamSlug = t.Slug
			ret = append(ret, &prom.Sample{
				Timestamp: prom.TimeFromUnix(opt.Time.Unix()),
				Metric:    makeLabels(),
				Value:     value(),
			})
		}
	}
	return ret, nil
}

func (c *FakeClient) QueryRange(ctx context.Context, environment string, query string, promRange promv1.Range) (prom.Value, promv1.Warnings, error) {
	matrix := prom.Matrix{}

	// start inclusive, end exclusive (Prometheus convention)
	for start := promRange.Start; start.Before(promRange.End); start = start.Add(promRange.Step) {
		vector, err := c.Query(ctx, environment, query, promclient.WithTime(start))
		if err != nil {
			return nil, nil, err
		}

		for _, sample := range vector {
			exists := slices.IndexFunc(matrix, func(i *prom.SampleStream) bool {
				return i.Metric.Equal(sample.Metric)
			})
			if exists >= 0 {
				matrix[exists].Values = append(matrix[exists].Values, prom.SamplePair{
					Timestamp: prom.TimeFromUnix(start.Unix()),
					Value:     sample.Value,
				})
				continue
			}
			matrix = append(matrix, &prom.SampleStream{
				Metric: sample.Metric,
				Values: []prom.SamplePair{{
					Timestamp: prom.TimeFromUnix(start.Unix()),
					Value:     sample.Value,
				}},
			})
		}
	}

	return matrix, nil, nil
}

func (c *FakeClient) selector(expr parser.Expr) (teamSlug slug.Slug, workload string, unit string, err error) {
	switch expr := expr.(type) {
	case *parser.VectorSelector:
		for _, m := range expr.LabelMatchers {
			if m.Type != labels.MatchEqual {
				continue
			}
			switch m.Name {
			case "namespace":
				teamSlug = slug.Slug(m.Value)
			case "app", "container":
				workload = m.Value
			case "unit":
				unit = m.Value
			}
		}

		if unit == "" {
			if strings.HasSuffix(expr.Name, "_bytes") {
				unit = "byte"
			}
			if strings.Contains(expr.Name, "_cpu_") {
				unit = "core"
			}
		}

	case *parser.Call:
		return c.selector(expr.Args[0])

	case *parser.MatrixSelector:
		return c.selector(expr.VectorSelector)

	case *parser.BinaryExpr:
		teamSlug, workload, unit, err = c.selector(expr.LHS)
		if err != nil {
			return "", "", "", err
		}

	case *parser.SubqueryExpr:
		return c.selector(expr.Expr)

	case *parser.NumberLiteral:
		// no-op

	default:
		return "", "", "", fmt.Errorf("selector: unexpected expression type %T", expr)
	}

	return teamSlug, workload, unit, nil
}

func (c *FakeClient) Rules(ctx context.Context, environment string, teamSlug slug.Slug) (promv1.RulesResult, error) {
	page, err := pagination.ParsePage(new(10000), nil, nil, nil)
	if err != nil {
		return promv1.RulesResult{}, err
	}
	teams, err := team.List(ctx, page, nil, nil)
	if err != nil {
		return promv1.RulesResult{}, err
	}

	now := c.now().Time()
	groups := make([]promv1.RuleGroup, 0, teams.PageInfo.TotalCount)

	for _, t := range teams.Nodes() {
		if t.Slug != teamSlug {
			continue
		}

		groupName := fmt.Sprintf("team-%s.rules", t.Slug)
		file := fmt.Sprintf("%s/%s/%s/e9441e95-9a6d-4d21-93cf-265e51aa0a67", environment, t.Slug, groupName)

		labelsFor := func(sev string) prom.LabelSet {
			return prom.LabelSet{
				teamLabelKey:  prom.LabelValue(t.Slug),
				"environment": prom.LabelValue(environment),
				"severity":    prom.LabelValue(sev),
			}
		}

		var rules promv1.Rules
		rules = append(rules, promv1.AlertingRule{
			Name:           "HighCPUSaturation",
			Query:          `avg by (namespace) (rate(container_cpu_usage_seconds_total{container!=""}[5m])) > 0.8`,
			Labels:         labelsFor("warning"),
			Annotations:    prom.LabelSet{"summary": "High CPU usage"},
			Health:         promv1.RuleHealthGood,
			LastEvaluation: now,
			Alerts:         []*promv1.Alert{},
		})
		rules = append(rules, promv1.AlertingRule{
			Name:           "HighMemoryUsage",
			Query:          `avg by (namespace) (container_memory_working_set_bytes{container!=""}) > 1.5e+9`,
			Labels:         labelsFor("critical"),
			Annotations:    prom.LabelSet{"summary": "High memory usage"},
			Health:         promv1.RuleHealthGood,
			LastEvaluation: now,
			Alerts:         []*promv1.Alert{},
		})
		rules = append(rules, promv1.AlertingRule{
			Name:           "HTTPErrorRateTooHigh",
			Query:          `sum(rate(http_requests_total{code=~"5.."}[5m])) by (namespace) / sum(rate(http_requests_total[5m])) by (namespace) > 0.05`,
			Labels:         labelsFor("high"),
			Annotations:    prom.LabelSet{"summary": "HTTP 5xx ratio > 5%"},
			Health:         promv1.RuleHealthGood,
			LastEvaluation: now,
			Alerts:         []*promv1.Alert{},
		})

		groups = append(groups, promv1.RuleGroup{
			Name:  groupName,
			File:  file,
			Rules: rules,
		})
	}

	return promv1.RulesResult{Groups: groups}, nil
}

func (c *FakeClient) RulesAll(ctx context.Context, teamSlug slug.Slug) (map[string]promv1.RulesResult, error) {
	out := make(map[string]promv1.RulesResult, len(c.environments))
	for _, env := range c.environments {
		res, err := c.Rules(ctx, env, teamSlug)
		if err != nil {
			return nil, err
		}

		out[env] = res
	}

	return out, nil
}

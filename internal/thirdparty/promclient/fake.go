package promclient

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload/application"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prom "github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	parser "github.com/prometheus/prometheus/promql/parser"
	"k8s.io/utils/ptr"
)

type FakeClient struct {
	environments []string
	random       *rand.Rand
	now          func() prom.Time
}

func NewFakeClient(environments []string, random *rand.Rand, nowFunc func() prom.Time) *FakeClient {
	if nowFunc == nil {
		nowFunc = prom.Now
	}

	if random == nil {
		random = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	}

	return &FakeClient{environments: environments, random: random, now: nowFunc}
}

func (c *FakeClient) QueryAll(ctx context.Context, query string, opts ...QueryOption) (map[string]prom.Vector, error) {
	ret := map[string]prom.Vector{}
	for _, env := range c.environments {
		v, err := c.Query(ctx, env, query, opts...)
		if err != nil {
			return nil, err
		}

		ret[env] = v
	}

	return ret, nil
}

func (c *FakeClient) Query(ctx context.Context, environment string, query string, opts ...QueryOption) (prom.Vector, error) {
	opt := queryOpts{
		time: c.now().Time(),
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
		labelsToCreate = append(labelsToCreate, "pod")
		teamSlug, workload, unit, err = c.selector(expr)

	default:
		return nil, fmt.Errorf("query: unexpected expression type %T", expr)
	}

	if err != nil {
		return nil, err
	}

	makeLabels := func() prom.Metric {
		lbls := prom.Metric{}
		for _, label := range labelsToCreate {
			switch label {
			case "namespace":
				lbls["namespace"] = prom.LabelValue(teamSlug)
			case "workload_namespace":
				lbls["workload_namespace"] = prom.LabelValue(teamSlug)
			case "container":
				lbls["container"] = prom.LabelValue(workload)
			case "pod":
				lbls["pod"] = prom.LabelValue(fmt.Sprintf("%s-%s", workload, "1"))
			}
		}
		return lbls
	}

	value := func() prom.SampleValue {
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
					Timestamp: prom.TimeFromUnix(opt.time.Unix()),
					Metric:    makeLabels(),
					Value:     value(),
				})
			}

			return ret, nil
		}
		ret = prom.Vector{
			{
				Timestamp: prom.TimeFromUnix(opt.time.Unix()),
				Metric:    makeLabels(),
				Value:     value(),
			},
		}
	} else {
		page, err := pagination.ParsePage(ptr.To(10000), nil, nil, nil)
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
				Timestamp: prom.TimeFromUnix(opt.time.Unix()),
				Metric:    makeLabels(),
				Value:     value(),
			})
		}
	}
	return ret, nil
}

func (c *FakeClient) QueryRange(ctx context.Context, environment string, query string, promRange promv1.Range) (prom.Value, promv1.Warnings, error) {
	matrix := prom.Matrix{}

	prevNow := c.now
	defer func() {
		c.now = prevNow
	}()

	for start := promRange.Start; start.Before(promRange.End); start = start.Add(promRange.Step) {
		c.now = func() prom.Time {
			return prom.TimeFromUnix(start.Unix())
		}

		vector, err := c.Query(ctx, environment, query)
		if err != nil {
			return nil, nil, err
		}

		for _, sample := range vector {
			exists := slices.IndexFunc(matrix, func(i *prom.SampleStream) bool {
				return i.Metric.Equal(sample.Metric)
			})
			if exists >= 0 {
				matrix[exists].Values = append(matrix[exists].Values, prom.SamplePair{Timestamp: c.now(), Value: sample.Value})
				continue
			} else {
				matrix = append(matrix, &prom.SampleStream{
					Metric: sample.Metric,
					Values: []prom.SamplePair{{Timestamp: c.now(), Value: sample.Value}},
				})
			}
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

	default:
		return "", "", "", fmt.Errorf("selector: unexpected expression type %T", expr)
	}

	return teamSlug, workload, unit, nil
}

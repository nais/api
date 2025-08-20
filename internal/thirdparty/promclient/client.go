package promclient

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prom "github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"github.com/sourcegraph/conc/pool"
)

type QueryClient interface {
	Query(ctx context.Context, environment string, query string, opts ...QueryOption) (prom.Vector, error)
	QueryAll(ctx context.Context, query string, opts ...QueryOption) (map[string]prom.Vector, error)
	QueryRange(ctx context.Context, environment string, query string, promRange promv1.Range) (prom.Value, promv1.Warnings, error)
}

type RulesClient interface {
	Rules(ctx context.Context, environment, team string) (promv1.RulesResult, error)
	RulesAll(ctx context.Context, team string) (map[string]promv1.RulesResult, error)
}

type AlertsClient interface {
	Alerts(ctx context.Context, environment, team string) (promv1.AlertsResult, error)
	AlertsAll(ctx context.Context, team string) (map[string]promv1.AlertsResult, error)
}

type Client interface {
	QueryClient
	RulesClient
	AlertsClient
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
	prometheuses map[string]promv1.API
	log          logrus.FieldLogger
}

func New(clusters []string, tenant string, log logrus.FieldLogger) (*RealClient, error) {
	proms := map[string]promv1.API{}

	for _, cluster := range clusters {
		client, err := api.NewClient(api.Config{Address: fmt.Sprintf("https://prometheus.%s.%s.cloud.nais.io", cluster, tenant)})
		if err != nil {
			return nil, err
		}

		proms[cluster] = promv1.NewAPI(client)
	}

	return &RealClient{
		prometheuses: proms,
		log:          log,
	}, nil
}

func (c *RealClient) QueryAll(ctx context.Context, query string, opts ...QueryOption) (map[string]prom.Vector, error) {
	type result struct {
		env string
		vec prom.Vector
	}
	wg := pool.NewWithResults[*result]().WithContext(ctx)

	for env := range c.prometheuses {
		wg.Go(func(ctx context.Context) (*result, error) {
			v, err := c.Query(ctx, env, query, opts...)
			if err != nil {
				c.log.WithError(err).Errorf("failed to query prometheus in %s", env)
				return nil, err
			}
			return &result{env: env, vec: v}, nil
		})
	}

	results, err := wg.Wait()
	if err != nil {
		return nil, err
	}

	ret := map[string]prom.Vector{}
	for _, res := range results {
		ret[res.env] = res.vec
	}

	return ret, nil
}

func (c *RealClient) Query(ctx context.Context, environmentName string, query string, opts ...QueryOption) (prom.Vector, error) {
	client, ok := c.prometheuses[environmentName]
	if !ok {
		return nil, fmt.Errorf("no prometheus client for environment %s", environmentName)
	}

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
	client, ok := c.prometheuses[environment]
	if !ok {
		return nil, nil, fmt.Errorf("no prometheus client for environment %s", environment)
	}

	return client.QueryRange(ctx, query, promRange)
}

const teamLabelKey = "team" // adjust if you use another key

func (c *RealClient) Rules(ctx context.Context, environment, team string) (promv1.RulesResult, error) {
	api, ok := c.prometheuses[environment]
	if !ok {
		return promv1.RulesResult{}, fmt.Errorf("no prometheus client for environment %s", environment)
	}
	res, err := api.Rules(ctx)
	if err != nil {
		return promv1.RulesResult{}, err
	}
	if team == "" {
		return res, nil
	}
	return filterRulesByTeam(res, team), nil
}

func (c *RealClient) RulesAll(ctx context.Context, team string) (map[string]promv1.RulesResult, error) {
	type item struct {
		env string
		res promv1.RulesResult
	}
	wg := pool.NewWithResults[*item]().WithContext(ctx)

	for env := range c.prometheuses {
		env := env
		wg.Go(func(ctx context.Context) (*item, error) {
			res, err := c.Rules(ctx, env, team)
			if err != nil {
				c.log.WithError(err).Errorf("failed to get rules in %s", env)
				return nil, err
			}
			return &item{env: env, res: res}, nil
		})
	}
	items, err := wg.Wait()
	if err != nil {
		return nil, err
	}
	out := make(map[string]promv1.RulesResult, len(items))
	for _, it := range items {
		out[it.env] = it.res
	}
	return out, nil
}

func (c *RealClient) Alerts(ctx context.Context, environment, team string) (promv1.AlertsResult, error) {
	api, ok := c.prometheuses[environment]
	if !ok {
		return promv1.AlertsResult{}, fmt.Errorf("no prometheus client for environment %s", environment)
	}
	res, err := api.Alerts(ctx)
	if err != nil {
		return promv1.AlertsResult{}, err
	}
	if team == "" {
		return res, nil
	}
	return filterAlertsByTeam(res, team), nil
}

func (c *RealClient) AlertsAll(ctx context.Context, team string) (map[string]promv1.AlertsResult, error) {
	type item struct {
		env string
		res promv1.AlertsResult
	}
	wg := pool.NewWithResults[*item]().WithContext(ctx)

	for env := range c.prometheuses {
		env := env
		wg.Go(func(ctx context.Context) (*item, error) {
			res, err := c.Alerts(ctx, env, team)
			if err != nil {
				c.log.WithError(err).Errorf("failed to get alerts in %s", env)
				return nil, err
			}
			return &item{env: env, res: res}, nil
		})
	}
	items, err := wg.Wait()
	if err != nil {
		return nil, err
	}
	out := make(map[string]promv1.AlertsResult, len(items))
	for _, it := range items {
		out[it.env] = it.res
	}
	return out, nil
}

func filterRulesByTeam(in promv1.RulesResult, team string) promv1.RulesResult {
	out := promv1.RulesResult{}
	out.Groups = make([]promv1.RuleGroup, 0, len(in.Groups))

	for _, g := range in.Groups {
		var filtered promv1.Rules
		for _, r := range g.Rules {
			if ar, ok := r.(promv1.AlertingRule); ok {
				if string(ar.Labels[prom.LabelName(teamLabelKey)]) == team {
					filtered = append(filtered, ar)
				}
			}
			// (Recording rules are ignored on purpose)
		}
		if len(filtered) > 0 {
			out.Groups = append(out.Groups, promv1.RuleGroup{
				Name:  g.Name,
				File:  g.File,
				Rules: filtered,
				// Skip Interval/LastEvaluation for cross-version compatibility
			})
		}
	}
	return out
}

func filterAlertsByTeam(in promv1.AlertsResult, team string) promv1.AlertsResult {
	out := promv1.AlertsResult{}
	for _, a := range in.Alerts {
		if string(a.Labels[prom.LabelName(teamLabelKey)]) == team {
			out.Alerts = append(out.Alerts, a)
		}
	}
	return out
}

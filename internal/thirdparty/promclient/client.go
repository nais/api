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

type Client interface {
	Query(ctx context.Context, environment string, query string, opts ...QueryOption) (prom.Vector, error)
	QueryAll(ctx context.Context, query string, opts ...QueryOption) (map[string]prom.Vector, error)
	QueryRange(ctx context.Context, environment string, query string, promRange promv1.Range) (prom.Value, promv1.Warnings, error)
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

package utilization

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

type ResourceUsageClient interface {
	query(ctx context.Context, environment string, query string) (prom.Vector, error)
	queryAll(ctx context.Context, query string) (map[string]prom.Vector, error)
	queryRange(ctx context.Context, environment string, query string, promRange promv1.Range) (prom.Value, promv1.Warnings, error)
}

type Client struct {
	prometheuses map[string]promv1.API
	log          logrus.FieldLogger
}

func NewClient(clusters []string, tenant string, log logrus.FieldLogger) (*Client, error) {
	proms := map[string]promv1.API{}

	for _, cluster := range clusters {
		client, err := api.NewClient(api.Config{Address: fmt.Sprintf("https://prometheus.%s.%s.cloud.nais.io", cluster, tenant)})
		if err != nil {
			return nil, err
		}

		proms[cluster] = promv1.NewAPI(client)
	}

	return &Client{
		prometheuses: proms,
		log:          log,
	}, nil
}

func (c *Client) queryAll(ctx context.Context, query string) (map[string]prom.Vector, error) {
	type result struct {
		env string
		vec prom.Vector
	}
	wg := pool.NewWithResults[*result]().WithContext(ctx)

	for env := range c.prometheuses {
		wg.Go(func(ctx context.Context) (*result, error) {
			v, err := c.query(ctx, env, query)
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

func (c *Client) query(ctx context.Context, environment string, query string) (prom.Vector, error) {
	client, ok := c.prometheuses[environment]
	if !ok {
		return nil, fmt.Errorf("no prometheus client for environment %s", environment)
	}

	v, warnings, err := client.Query(ctx, query, time.Now().Add(-5*time.Minute))
	if err != nil {
		return nil, err
	}

	if len(warnings) > 0 {
		return nil, fmt.Errorf("prometheus query warnings: %s", strings.Join(warnings, ", "))
	}

	vector, ok := v.(prom.Vector)
	if !ok {
		return nil, fmt.Errorf("expected prometheus vector, got %T", v)
	}

	return vector, nil
}

func (c *Client) queryRange(ctx context.Context, environment string, query string, promRange promv1.Range) (prom.Value, promv1.Warnings, error) {
	client, ok := c.prometheuses[environment]
	if !ok {
		return nil, nil, fmt.Errorf("no prometheus client for environment %s", environment)
	}

	return client.QueryRange(ctx, query, promRange)
}
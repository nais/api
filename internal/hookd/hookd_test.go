package hookd_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/nais/api/internal/config"
	"github.com/nais/api/internal/hookd"
	httptest "github.com/nais/api/internal/test"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/exporters/prometheus"
	met "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
)

const token = "token"

func TestClient(t *testing.T) {
	ctx := context.Background()

	logger, _ := test.NewNullLogger()
	meter, err := getMetricMeter()
	assert.NoError(t, err)

	counter, err := meter.Int64Counter("errors")
	assert.NoError(t, err)

	cfg := config.Hookd{
		PSK: token,
	}

	t.Run("empty response when fetching deployments", func(t *testing.T) {
		hookdServer := httptest.NewHttpServerWithHandlers(t, []http.HandlerFunc{
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, token, r.Header.Get("X-PSK"))
				resp, _ := json.Marshal(hookd.DeploymentsResponse{
					Deployments: []hookd.Deploy{},
				})
				w.Write(resp)
			},
		})

		cfg.Endpoint = hookdServer.URL
		client := hookd.New(cfg, counter, logger)

		deployments, err := client.Deployments(ctx)
		assert.NoError(t, err)
		assert.Empty(t, deployments)
	})

	t.Run("fetch deployment with request options", func(t *testing.T) {
		hookdServer := httptest.NewHttpServerWithHandlers(t, []http.HandlerFunc{
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "team", r.URL.Query().Get("team"))
				resp, _ := json.Marshal(hookd.DeploymentsResponse{
					Deployments: []hookd.Deploy{},
				})
				w.Write(resp)
			},
		})

		cfg.Endpoint = hookdServer.URL
		client := hookd.New(cfg, counter, logger)

		deployments, err := client.Deployments(ctx, hookd.WithTeam("team"))
		assert.NoError(t, err)
		assert.Empty(t, deployments)
	})

	t.Run("fetch deployments", func(t *testing.T) {
		hookdServer := httptest.NewHttpServerWithHandlers(t, []http.HandlerFunc{
			func(w http.ResponseWriter, r *http.Request) {
				d := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
				resp, _ := json.Marshal(hookd.DeploymentsResponse{
					Deployments: []hookd.Deploy{
						{
							DeploymentInfo: hookd.DeploymentInfo{
								ID:      "1",
								Created: d,
							},
						},
						{
							DeploymentInfo: hookd.DeploymentInfo{
								ID:      "2",
								Created: d.AddDate(0, 1, 0),
							},
						},
						{
							DeploymentInfo: hookd.DeploymentInfo{
								ID:      "3",
								Created: d.AddDate(0, 2, 0),
							},
						},
					},
				})
				w.Write(resp)
			},
		})

		cfg.Endpoint = hookdServer.URL
		client := hookd.New(cfg, counter, logger)

		deployments, err := client.Deployments(ctx, hookd.WithTeam("team"))
		assert.NoError(t, err)
		assert.Len(t, deployments, 3)
		assert.Equal(t, "3", deployments[0].DeploymentInfo.ID)
		assert.Equal(t, "2", deployments[1].DeploymentInfo.ID)
		assert.Equal(t, "1", deployments[2].DeploymentInfo.ID)
	})

	t.Run("get deploykey errors when error is returned from backend", func(t *testing.T) {
		hookdServer := httptest.NewHttpServerWithHandlers(t, []http.HandlerFunc{
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		})

		cfg.Endpoint = hookdServer.URL
		client := hookd.New(cfg, counter, logger)

		deployments, err := client.DeployKey(ctx, "team")
		assert.Nil(t, deployments)
		assert.ErrorContains(t, err, "Internal Server Error")
	})

	t.Run("get deploykey errors when response from server is invalid", func(t *testing.T) {
		hookdServer := httptest.NewHttpServerWithHandlers(t, []http.HandlerFunc{
			func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("some string"))
			},
		})

		cfg.Endpoint = hookdServer.URL
		client := hookd.New(cfg, counter, logger)

		key, err := client.DeployKey(ctx, "team")
		assert.Nil(t, key)
		assert.ErrorContains(t, err, "invalid reply from server")
	})

	t.Run("get deploykey", func(t *testing.T) {
		hookdServer := httptest.NewHttpServerWithHandlers(t, []http.HandlerFunc{
			func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"team":"some-team", "key":"some-key"}`))
			},
		})

		cfg.Endpoint = hookdServer.URL
		client := hookd.New(cfg, counter, logger)

		key, err := client.DeployKey(ctx, "team")
		assert.NoError(t, err)
		assert.Equal(t, "some-team", key.Team)
		assert.Equal(t, "some-key", key.Key)
	})
}

func TestRequestOptions(t *testing.T) {
	const team = "team"
	const cluster = "cluster"
	const limit = 42
	ignoreTeams := []string{"team1", "team2"}

	r, err := http.NewRequest(http.MethodGet, "http://localhost", nil)
	assert.NoError(t, err)

	hookd.WithTeam(team)(r)
	hookd.WithCluster(cluster)(r)
	hookd.WithLimit(limit)(r)
	hookd.WithIgnoreTeams(ignoreTeams...)(r)

	assert.Equal(t, team, r.URL.Query().Get("team"))
	assert.Equal(t, cluster, r.URL.Query().Get("cluster"))
	assert.Equal(t, strconv.FormatInt(limit, 10), r.URL.Query().Get("limit"))
	assert.Equal(t, "team1,team2", r.URL.Query().Get("ignoreTeam"))
}

func getMetricMeter() (met.Meter, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("create prometheus exporter: %w", err)
	}

	provider := metric.NewMeterProvider(metric.WithReader(exporter))
	return provider.Meter("hookd_test"), nil
}

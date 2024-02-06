package hookd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nais/api/internal/slug"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type Client interface {
	Deployments(ctx context.Context, opts ...RequestOption) ([]Deploy, error)
	ChangeDeployKey(ctx context.Context, team string) (*DeployKey, error)
	DeployKey(ctx context.Context, team string) (*DeployKey, error)
}

type client struct {
	endpoint   string
	httpClient *httpClient
	log        logrus.FieldLogger
}

type DeploymentsResponse struct {
	Deployments []Deploy `json:"deployments"`
}

type Deploy struct {
	DeploymentInfo DeploymentInfo `json:"deployment"`
	Statuses       []Status       `json:"statuses"`
	Resources      []Resource     `json:"resources"`
}

type DeploymentInfo struct {
	ID               string    `json:"id"`
	Team             slug.Slug `json:"team"`
	Cluster          string    `json:"cluster"`
	Created          time.Time `json:"created"`
	GithubRepository string    `json:"githubRepository"`
}

type Status struct {
	ID      string    `json:"id"`
	Status  string    `json:"status"`
	Message string    `json:"message"`
	Created time.Time `json:"created"`
}

type Resource struct {
	ID        string `json:"id"`
	Group     string `json:"group"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Namespace string `json:"namespace"`
}

type DeployKey struct {
	Team    string    `json:"team"`
	Key     string    `json:"key"`
	Expires time.Time `json:"expires"`
	Created time.Time `json:"created"`
}

type RequestOption func(*http.Request)

func WithTeam(team string) RequestOption {
	return func(req *http.Request) {
		q := req.URL.Query()
		q.Set("team", team)
		req.URL.RawQuery = q.Encode()
	}
}

func WithCluster(cluster string) RequestOption {
	return func(req *http.Request) {
		q := req.URL.Query()
		q.Set("cluster", cluster)
		req.URL.RawQuery = q.Encode()
	}
}

func WithLimit(limit int) RequestOption {
	return func(req *http.Request) {
		q := req.URL.Query()
		q.Set("limit", strconv.Itoa(limit))
		req.URL.RawQuery = q.Encode()
	}
}

func WithIgnoreTeams(teams ...string) RequestOption {
	return func(req *http.Request) {
		q := req.URL.Query()
		q.Set("ignoreTeam", strings.Join(teams, ","))
		req.URL.RawQuery = q.Encode()
	}
}

// New creates a new hookd client
func New(endpoint, psk string, log logrus.FieldLogger) Client {
	return &client{
		endpoint: endpoint,
		httpClient: &httpClient{
			client: &http.Client{
				Transport: otelhttp.NewTransport(http.DefaultTransport),
			},
			psk: psk,
		},
		log: log,
	}
}

// Deployments returns a list of deployments from hookd
func (c *client) Deployments(ctx context.Context, opts ...RequestOption) ([]Deploy, error) {
	url := c.endpoint + "/internal/api/v1/console/deployments"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, c.error(ctx, err, "create request for hookd")
	}

	for _, opt := range opts {
		opt(req)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, c.error(ctx, err, "calling hookd")
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.log.WithError(err).Error("closing response body")
		}
	}()

	var deploymentsResponse DeploymentsResponse

	if err := json.NewDecoder(resp.Body).Decode(&deploymentsResponse); err != nil {
		return nil, c.error(ctx, err, "decoding response from hookd")
	}

	ret := deploymentsResponse.Deployments

	sort.Slice(ret, func(i, j int) bool {
		return ret[i].DeploymentInfo.Created.After(ret[j].DeploymentInfo.Created)
	})

	return ret, nil
}

// ChangeDeployKey changes the deploy key for a team
func (c *client) ChangeDeployKey(ctx context.Context, team string) (*DeployKey, error) {
	url := fmt.Sprintf("%s/internal/api/v1/console/apikey/%s", c.endpoint, team)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, c.error(ctx, err, "create request for deploy key API")
	}

	resp, err := c.httpClient.Do(request)
	if err != nil {
		return nil, c.error(ctx, err, "calling hookd")
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.log.WithError(err).Error("closing response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, c.error(ctx, fmt.Errorf("deploy key API returned %s", resp.Status), "deploy key API returned non-200")
	}

	return c.DeployKey(ctx, team)
}

// DeployKey returns a deploy key for a team
func (c *client) DeployKey(ctx context.Context, team string) (*DeployKey, error) {
	url := fmt.Sprintf("%s/internal/api/v1/console/apikey/%s", c.endpoint, team)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, c.error(ctx, err, "create request for deploy key API")
	}

	resp, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.log.WithError(err).Error("closing response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, c.error(ctx, fmt.Errorf("deploy key API returned %s", resp.Status), "deploy key API returned non-200")
	}

	data, _ := io.ReadAll(resp.Body)
	ret := &DeployKey{}
	err = json.Unmarshal(data, ret)
	if err != nil {
		return nil, c.error(ctx, err, "invalid reply from server")
	}

	return ret, nil
}

// error increments the error counter, logs an error, and returns an error instance
func (c *client) error(ctx context.Context, err error, msg string) error {
	c.log.WithError(err).Error(msg)
	return fmt.Errorf("%s: %w", msg, err)
}

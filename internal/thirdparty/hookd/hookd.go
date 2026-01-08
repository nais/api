package hookd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var ErrNotFound = fmt.Errorf("deploy key not found")

type Client interface {
	ChangeDeployKey(ctx context.Context, team string) (*DeployKey, error)
	DeployKey(ctx context.Context, team string) (*DeployKey, error)
}

type client struct {
	endpoint   string
	httpClient *httpClient
	log        logrus.FieldLogger
}

type DeployKey struct {
	Team    string    `json:"team"`
	Key     string    `json:"key"`
	Expires time.Time `json:"expires"`
	Created time.Time `json:"created"`
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
		if resp.StatusCode == http.StatusNotFound {
			// Skip logging not found errors as they are expected for now teams
			return nil, ErrNotFound
		}

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

func (c *client) error(_ context.Context, err error, msg string) error {
	c.log.WithError(err).Error(msg)
	return fmt.Errorf("%s: %w", msg, err)
}

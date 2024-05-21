package unleash

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	bifrost "github.com/nais/bifrost/pkg/unleash"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"io"
	"net/http"
	"strings"
)

type BifrostClient interface {
	Post(ctx context.Context, path string, v any) (*http.Response, error)
	WithClient(client *http.Client)
}

type bifrostClient struct {
	url    string
	client *http.Client
	log    logrus.FieldLogger
}

func NewBifrostClient(url string, log logrus.FieldLogger) BifrostClient {
	return &bifrostClient{
		url: url,
		client: &http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
		log: log,
	}
}

func (b *bifrostClient) WithClient(client *http.Client) {
	b.client = client
}

func (m *Manager) NewUnleash(ctx context.Context, name string, allowedTeams []string) error {
	// TODO implement auth, set iap header with actor from context or use psk - must update bifrost to support this
	teams := strings.Join(allowedTeams, ",")
	bi := bifrost.UnleashConfig{
		Name:         name,
		AllowedTeams: teams,
	}
	_, err := m.bifrostClient.Post(ctx, "/unleash/new", bi)
	if err != nil {
		return err
	}
	return nil
}

func (m *Manager) UpdateUnleash(ctx context.Context, name string, allowedTeams []string) error {
	teams := strings.Join(allowedTeams, ",")
	bi := bifrost.UnleashConfig{
		Name:         name,
		AllowedTeams: teams,
	}
	_, err := m.bifrostClient.Post(ctx, "/unleash/edit", bi)
	if err != nil {
		return err
	}
	return nil
}

func (b *bifrostClient) Post(ctx context.Context, path string, v any) (*http.Response, error) {
	js, err := json.Marshal(v)
	if err != nil {
		return nil, b.error(err, "marshal unleash config")
	}

	body := io.NopCloser(bytes.NewReader(js))
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, b.url+path, body)
	if err != nil {
		return nil, b.error(err, "create request")
	}

	resp, err := b.client.Do(request)
	if err != nil {
		return nil, b.error(err, "calling bifrost")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, b.error(fmt.Errorf("bifrost returned %s", resp.Status), "bifrost returned non-200")
	}
	return resp, nil
}

func (b *bifrostClient) error(err error, msg string) error {
	b.log.WithError(err).Error(msg)
	return fmt.Errorf("%s: %w", msg, err)
}

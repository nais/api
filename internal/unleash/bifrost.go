package unleash

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/nais/api/internal/graph/model"
	bifrost "github.com/nais/bifrost/pkg/unleash"
	unleash_nais_io_v1 "github.com/nais/unleasherator/api/v1"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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

func (m *Manager) NewUnleash(ctx context.Context, name string, allowedTeams []string) (*model.Unleash, error) {
	if !m.settings.unleashEnabled {
		return &model.Unleash{
			Enabled: false,
		}, fmt.Errorf("unleash is not enabled")
	}

	// TODO implement auth, set iap header with actor from context or use psk - must update bifrost to support this
	teams := strings.Join(allowedTeams, ",")
	bi := bifrost.UnleashConfig{
		Name:             name,
		AllowedTeams:     teams,
		EnableFederation: true,
		AllowedClusters:  "dev-gcp,prod-gcp,dev-fss,prod-fss",
	}
	unleashResponse, err := m.bifrostClient.Post(ctx, "/unleash/new", bi)
	if err != nil {
		return nil, err
	}

	var unleashInstance unleash_nais_io_v1.Unleash
	err = json.NewDecoder(unleashResponse.Body).Decode(&unleashInstance)
	if err != nil {
		return nil, fmt.Errorf("decoding unleash instance: %w", err)
	}

	return &model.Unleash{
		Instance: model.ToUnleashInstance(&unleashInstance),
		Enabled:  true,
	}, nil
}

func (m *Manager) UpdateUnleash(ctx context.Context, name string, allowedTeams []string) (*model.Unleash, error) {
	if !m.settings.unleashEnabled {
		return &model.Unleash{Enabled: false}, fmt.Errorf("unleash is not enabled")
	}

	teams := strings.Join(allowedTeams, ",")
	bi := bifrost.UnleashConfig{
		Name:         name,
		AllowedTeams: teams,
	}
	unleashResponse, err := m.bifrostClient.Post(ctx, fmt.Sprintf("/unleash/%s/edit", name), bi)
	if err != nil {
		return nil, err
	}

	var unleashInstance unleash_nais_io_v1.Unleash
	err = json.NewDecoder(unleashResponse.Body).Decode(&unleashInstance)
	if err != nil {
		return nil, fmt.Errorf("decoding unleash instance: %w", err)
	}

	return &model.Unleash{
		Instance: model.ToUnleashInstance(&unleashInstance),
		Enabled:  true,
	}, nil
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

	request.Header.Set("Content-Type", "application/json")

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

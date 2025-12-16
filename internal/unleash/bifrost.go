package unleash

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type BifrostClient interface {
	Post(ctx context.Context, path string, v any) (*http.Response, error)
	Put(ctx context.Context, path string, v any) (*http.Response, error)
	Get(ctx context.Context, path string) (*http.Response, error)
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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, b.error(fmt.Errorf("bifrost returned %s", resp.Status), "bifrost returned non-2xx")
	}
	return resp, nil
}

func (b *bifrostClient) Put(ctx context.Context, path string, v any) (*http.Response, error) {
	js, err := json.Marshal(v)
	if err != nil {
		return nil, b.error(err, "marshal unleash config")
	}

	body := io.NopCloser(bytes.NewReader(js))
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, b.url+path, body)
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

func (b *bifrostClient) Get(ctx context.Context, path string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, b.url+path, nil)
	if err != nil {
		return nil, b.error(err, "create request")
	}

	request.Header.Set("Accept", "application/json")

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

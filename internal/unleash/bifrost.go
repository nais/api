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
	Delete(ctx context.Context, path string) (*http.Response, error)
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
		return nil, b.handleErrorResponse(resp, "POST", path)
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
		return nil, b.handleErrorResponse(resp, "PUT", path)
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
		return nil, b.handleErrorResponse(resp, "GET", path)
	}
	return resp, nil
}

func (b *bifrostClient) Delete(ctx context.Context, path string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, b.url+path, nil)
	if err != nil {
		return nil, b.error(err, "create request")
	}

	resp, err := b.client.Do(request)
	if err != nil {
		return nil, b.error(err, "calling bifrost")
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return nil, b.handleErrorResponse(resp, "DELETE", path)
	}
	return resp, nil
}

// handleErrorResponse parses and returns a structured error from bifrost's error response
func (b *bifrostClient) handleErrorResponse(resp *http.Response, method, path string) error {
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		b.log.WithError(err).WithFields(logrus.Fields{
			"method":      method,
			"path":        path,
			"status_code": resp.StatusCode,
		}).Error("failed to read bifrost error response body")
		return fmt.Errorf("bifrost %s %s returned %s", method, path, resp.Status)
	}

	var bifrostErr BifrostV1ErrorResponse
	if err := json.Unmarshal(bodyBytes, &bifrostErr); err != nil {
		// If we can't parse the error response, return the raw status
		b.log.WithFields(logrus.Fields{
			"method":      method,
			"path":        path,
			"status_code": resp.StatusCode,
			"body":        string(bodyBytes),
		}).Error("bifrost returned error")
		return fmt.Errorf("bifrost %s %s returned %s", method, path, resp.Status)
	}

	// Log the structured error
	b.log.WithFields(logrus.Fields{
		"method":        method,
		"path":          path,
		"status_code":   resp.StatusCode,
		"error_type":    bifrostErr.Error,
		"error_message": bifrostErr.Message,
		"error_details": bifrostErr.Details,
	}).Error("bifrost returned error")

	// Return a user-friendly error message
	if bifrostErr.Message != "" {
		return fmt.Errorf("bifrost: %s", bifrostErr.Message)
	}
	if bifrostErr.Error != "" {
		return fmt.Errorf("bifrost: %s", bifrostErr.Error)
	}
	return fmt.Errorf("bifrost %s %s returned %s", method, path, resp.Status)
}

func (b *bifrostClient) error(err error, msg string) error {
	b.log.WithError(err).Error(msg)
	return fmt.Errorf("%s: %w", msg, err)
}

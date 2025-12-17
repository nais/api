package unleash

import (
	"context"
	"fmt"
	"net/http"

	"github.com/nais/api/internal/unleash/bifrostclient"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// BifrostClient provides a high-level interface for interacting with the Bifrost API.
// It wraps the generated OpenAPI client and provides error handling and logging.
type BifrostClient interface {
	CreateInstance(ctx context.Context, req bifrostclient.UnleashConfigRequest) (*bifrostclient.CreateInstanceResponse, error)
	UpdateInstance(ctx context.Context, name string, req bifrostclient.UnleashConfigRequest) (*bifrostclient.UpdateInstanceResponse, error)
	GetInstance(ctx context.Context, name string) (*bifrostclient.GetInstanceResponse, error)
	DeleteInstance(ctx context.Context, name string) (*bifrostclient.DeleteInstanceResponse, error)
	ListInstances(ctx context.Context) (*bifrostclient.ListInstancesResponse, error)
	ListChannels(ctx context.Context) (*bifrostclient.ListChannelsResponse, error)
	GetChannel(ctx context.Context, name string) (*bifrostclient.GetChannelResponse, error)
}

type bifrostClientImpl struct {
	client bifrostclient.ClientWithResponsesInterface
	log    logrus.FieldLogger
}

// NewBifrostClient creates a new BifrostClient with the given base URL and logger.
// The client uses OpenTelemetry-instrumented HTTP transport for tracing.
func NewBifrostClient(baseURL string, log logrus.FieldLogger) BifrostClient {
	httpClient := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	client, err := bifrostclient.NewClientWithResponses(baseURL, bifrostclient.WithHTTPClient(httpClient))
	if err != nil {
		// This should only fail if the base URL is invalid
		log.WithError(err).Fatal("failed to create bifrost client")
	}

	return &bifrostClientImpl{
		client: client,
		log:    log,
	}
}

// NewBifrostClientWithInterface creates a new BifrostClient using a provided ClientWithResponsesInterface.
// This is useful for testing with mock clients.
func NewBifrostClientWithInterface(client bifrostclient.ClientWithResponsesInterface, log logrus.FieldLogger) BifrostClient {
	return &bifrostClientImpl{
		client: client,
		log:    log,
	}
}

func (b *bifrostClientImpl) CreateInstance(ctx context.Context, req bifrostclient.UnleashConfigRequest) (*bifrostclient.CreateInstanceResponse, error) {
	resp, err := b.client.CreateInstanceWithResponse(ctx, req)
	if err != nil {
		return nil, b.logError(err, "CreateInstance", "calling bifrost")
	}

	if resp.JSON201 != nil {
		return resp, nil
	}

	return nil, b.handleErrorResponse(resp.HTTPResponse, resp.JSON400, resp.JSON500, "CreateInstance")
}

func (b *bifrostClientImpl) UpdateInstance(ctx context.Context, name string, req bifrostclient.UnleashConfigRequest) (*bifrostclient.UpdateInstanceResponse, error) {
	resp, err := b.client.UpdateInstanceWithResponse(ctx, name, req)
	if err != nil {
		return nil, b.logError(err, "UpdateInstance", "calling bifrost")
	}

	if resp.JSON200 != nil {
		return resp, nil
	}

	return nil, b.handleUpdateErrorResponse(resp)
}

func (b *bifrostClientImpl) GetInstance(ctx context.Context, name string) (*bifrostclient.GetInstanceResponse, error) {
	resp, err := b.client.GetInstanceWithResponse(ctx, name)
	if err != nil {
		return nil, b.logError(err, "GetInstance", "calling bifrost")
	}

	if resp.JSON200 != nil {
		return resp, nil
	}

	if resp.JSON404 != nil {
		return nil, b.formatBifrostError(resp.JSON404)
	}

	return nil, fmt.Errorf("bifrost GetInstance returned %s", resp.Status())
}

func (b *bifrostClientImpl) DeleteInstance(ctx context.Context, name string) (*bifrostclient.DeleteInstanceResponse, error) {
	resp, err := b.client.DeleteInstanceWithResponse(ctx, name)
	if err != nil {
		return nil, b.logError(err, "DeleteInstance", "calling bifrost")
	}

	// Success is 204 No Content, which means no JSON body
	if resp.StatusCode() == http.StatusNoContent || resp.StatusCode() == http.StatusOK {
		return resp, nil
	}

	if resp.JSON404 != nil {
		return nil, b.formatBifrostError(resp.JSON404)
	}

	if resp.JSON500 != nil {
		return nil, b.formatBifrostError(resp.JSON500)
	}

	return nil, fmt.Errorf("bifrost DeleteInstance returned %s", resp.Status())
}

func (b *bifrostClientImpl) ListInstances(ctx context.Context) (*bifrostclient.ListInstancesResponse, error) {
	resp, err := b.client.ListInstancesWithResponse(ctx)
	if err != nil {
		return nil, b.logError(err, "ListInstances", "calling bifrost")
	}

	if resp.JSON200 != nil {
		return resp, nil
	}

	if resp.JSON500 != nil {
		return nil, b.formatBifrostError(resp.JSON500)
	}

	return nil, fmt.Errorf("bifrost ListInstances returned %s", resp.Status())
}

func (b *bifrostClientImpl) ListChannels(ctx context.Context) (*bifrostclient.ListChannelsResponse, error) {
	resp, err := b.client.ListChannelsWithResponse(ctx)
	if err != nil {
		return nil, b.logError(err, "ListChannels", "calling bifrost")
	}

	if resp.JSON200 != nil {
		return resp, nil
	}

	if resp.JSON500 != nil {
		return nil, b.formatBifrostError(resp.JSON500)
	}

	return nil, fmt.Errorf("bifrost ListChannels returned %s", resp.Status())
}

func (b *bifrostClientImpl) GetChannel(ctx context.Context, name string) (*bifrostclient.GetChannelResponse, error) {
	resp, err := b.client.GetChannelWithResponse(ctx, name)
	if err != nil {
		return nil, b.logError(err, "GetChannel", "calling bifrost")
	}

	if resp.JSON200 != nil {
		return resp, nil
	}

	if resp.JSON404 != nil {
		return nil, b.formatBifrostError(resp.JSON404)
	}

	return nil, fmt.Errorf("bifrost GetChannel returned %s", resp.Status())
}

func (b *bifrostClientImpl) handleErrorResponse(resp *http.Response, json400, json500 *bifrostclient.ErrorResponse, operation string) error {
	if json400 != nil {
		b.logBifrostError(operation, resp.StatusCode, json400)
		return b.formatBifrostError(json400)
	}

	if json500 != nil {
		b.logBifrostError(operation, resp.StatusCode, json500)
		return b.formatBifrostError(json500)
	}

	b.log.WithFields(logrus.Fields{
		"operation":   operation,
		"status_code": resp.StatusCode,
	}).Error("bifrost returned unexpected error")

	return fmt.Errorf("bifrost %s returned %s", operation, resp.Status)
}

func (b *bifrostClientImpl) handleUpdateErrorResponse(resp *bifrostclient.UpdateInstanceResponse) error {
	if resp.JSON400 != nil {
		b.logBifrostError("UpdateInstance", resp.StatusCode(), resp.JSON400)
		return b.formatBifrostError(resp.JSON400)
	}

	if resp.JSON404 != nil {
		b.logBifrostError("UpdateInstance", resp.StatusCode(), resp.JSON404)
		return b.formatBifrostError(resp.JSON404)
	}

	if resp.JSON500 != nil {
		b.logBifrostError("UpdateInstance", resp.StatusCode(), resp.JSON500)
		return b.formatBifrostError(resp.JSON500)
	}

	return fmt.Errorf("bifrost UpdateInstance returned %s", resp.Status())
}

func (b *bifrostClientImpl) logBifrostError(operation string, statusCode int, errResp *bifrostclient.ErrorResponse) {
	b.log.WithFields(logrus.Fields{
		"operation":     operation,
		"status_code":   statusCode,
		"error_type":    errResp.Error,
		"error_message": errResp.Message,
		"error_details": errResp.Details,
	}).Error("bifrost returned error")
}

func (b *bifrostClientImpl) formatBifrostError(errResp *bifrostclient.ErrorResponse) error {
	if errResp.Message != "" {
		return fmt.Errorf("bifrost: %s", errResp.Message)
	}
	if errResp.Error != "" {
		return fmt.Errorf("bifrost: %s", errResp.Error)
	}
	return fmt.Errorf("bifrost error: status %d", errResp.StatusCode)
}

func (b *bifrostClientImpl) logError(err error, operation, msg string) error {
	b.log.WithError(err).WithField("operation", operation).Error(msg)
	return fmt.Errorf("%s: %w", msg, err)
}

package servicemaintenance_test

import (
	"context"
	"io"
	"sync/atomic"
	"testing"

	aivenproject "github.com/aiven/go-client-codegen/handler/project"
	aivenservice "github.com/aiven/go-client-codegen/handler/service"
	"github.com/nais/api/internal/servicemaintenance"
	"github.com/sirupsen/logrus"
)

type countingAivenClient struct{ calls atomic.Int64 }

func (c *countingAivenClient) ServiceGet(context.Context, string, string, ...[2]string) (*aivenservice.ServiceGetOut, error) {
	c.calls.Add(1)
	return &aivenservice.ServiceGetOut{
		Maintenance: &aivenservice.MaintenanceOut{
			Dow:  aivenservice.MaintenanceDowTypeSunday,
			Time: "12:34:56",
		},
	}, nil
}

func (c *countingAivenClient) ServiceMaintenanceStart(context.Context, string, string) error {
	return nil
}

func (c *countingAivenClient) ProjectAlertsList(context.Context, string) ([]aivenproject.AlertOut, error) {
	return nil, nil
}

// Two maintenance fields on one instance are one question to Aiven, which the loader collapses only
// if two equal keys compare equal.
func TestMaintenanceLoaderDeduplicatesEqualKeys(t *testing.T) {
	client := &countingAivenClient{}

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	mgr, err := servicemaintenance.NewManager(context.Background(), client, logrus.NewEntry(logger))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	ctx := servicemaintenance.NewLoaderContext(context.Background(), mgr, logger)
	key := servicemaintenance.AivenDataLoaderKey{Project: "nav-dev", ServiceName: "valkey-myteam-cache"}

	if _, err := servicemaintenance.GetAivenMaintenanceWindow(ctx, key); err != nil {
		t.Fatalf("GetAivenMaintenanceWindow() error = %v", err)
	}
	if _, err := servicemaintenance.GetAivenMaintenanceUpdates[servicemaintenance.ValkeyMaintenanceUpdate](ctx, key); err != nil {
		t.Fatalf("GetAivenMaintenanceUpdates() error = %v", err)
	}

	if got := client.calls.Load(); got != 1 {
		t.Errorf("ServiceGet called %d times, want 1", got)
	}
}

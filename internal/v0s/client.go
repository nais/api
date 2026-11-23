// Package v0s adapts the v0s gRPC contract without changing the legacy v13s API.
package v0s

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/apierror"
	protoapi "github.com/nais/v0s-api/v0s/v1"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Manager struct {
	Vulnerabilities     protoapi.VulnerabilityServiceClient
	Suppressions        protoapi.SuppressionServiceClient
	SuppressionsEnabled bool
	conn                *grpc.ClientConn
}

// NewManager creates a lazy connection. An unset endpoint disables only the new API.
func NewManager(endpoint string, suppressionsEnabled bool) (*Manager, error) {
	m := &Manager{SuppressionsEnabled: suppressionsEnabled}
	if endpoint == "" {
		return m, nil
	}
	conn, err := grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("create v0s client: %w", err)
	}
	m.conn = conn
	m.Vulnerabilities = protoapi.NewVulnerabilityServiceClient(conn)
	m.Suppressions = protoapi.NewSuppressionServiceClient(conn)
	return m, nil
}

func (m *Manager) Close() error {
	if m == nil || m.conn == nil {
		return nil
	}
	return m.conn.Close()
}

func (m *Manager) available() error {
	if m == nil || m.Vulnerabilities == nil {
		return apierror.Errorf("Vulnerability data is not configured.")
	}
	return nil
}

func WithManager(ctx context.Context, m *Manager) context.Context {
	return context.WithValue(ctx, managerKey, m)
}

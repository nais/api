package v0s_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/v0s"
	"github.com/nais/api/internal/v0s/protoapi"
	inventoryapi "github.com/nais/api/pkg/apiclient/protoapi"
	"google.golang.org/grpc"
)

type advisoryServer struct {
	protoapi.UnimplementedVulnerabilityServiceServer
}

func (advisoryServer) GetVulnerability(_ context.Context, request *protoapi.GetVulnerabilityRequest) (*protoapi.GetVulnerabilityResponse, error) {
	return &protoapi.GetVulnerabilityResponse{Vulnerability: &protoapi.Vulnerability{
		Id: request.GetId(), Cvss: []*protoapi.Cvss{{Source: "source", Version: "3.x"}},
	}}, nil
}

func TestClientReadsThroughGRPCWithoutInventoryDependency(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	protoapi.RegisterVulnerabilityServiceServer(server, advisoryServer{})
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		if err := <-done; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Error(err)
		}
	})
	m, err := v0s.NewManager(listener.Addr().String(), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ctx = v0s.WithManager(authz.ContextWithActor(ctx, actor{}, nil), m)
	ctx = v0s.NewLoaderContext(ctx, func(context.Context) (*inventoryapi.ListWorkloadImagesResponse, error) {
		t.Error("advisory read unexpectedly depends on inventory")
		return nil, errors.New("inventory unavailable")
	})
	got, err := v0s.VulnerabilityByIdentifier(ctx, "CVE-1")
	if err != nil || got.Identifier != "CVE-1" || got.Cvss[0].Score != nil {
		t.Fatalf("gRPC advisory: %+v, %v", got, err)
	}
}

func TestClientInitializationDoesNotRequireRunningV0s(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	m, err := v0s.NewManager(endpoint, false)
	if err != nil {
		t.Fatalf("client initialization contacted the unavailable backend: %v", err)
	}
	defer m.Close()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if _, err := m.Vulnerabilities.GetVulnerability(ctx, &protoapi.GetVulnerabilityRequest{Id: "CVE-1"}); err == nil {
		t.Fatal("unavailable backend returned a successful read")
	}
}

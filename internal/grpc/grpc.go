package grpc

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/grpc/grpcagent"
	"github.com/nais/api/internal/grpc/grpcagent/agent/chat"
	"github.com/nais/api/internal/grpc/grpcagent/agent/rag"
	"github.com/nais/api/internal/grpc/grpcdeployment"
	"github.com/nais/api/internal/grpc/grpcreconciler"
	"github.com/nais/api/internal/grpc/grpcteam"
	"github.com/nais/api/internal/grpc/grpcuser"
	"github.com/nais/api/pkg/apiclient/protoapi"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

// AgentConfig holds configuration for the agent gRPC service.
type AgentConfig struct {
	ChatClient chat.StreamingClient
	RAGClient  rag.DocumentSearcher
	NaisAPIURL string
}

// Config holds configuration for the gRPC server.
type Config struct {
	ListenAddress string
	Pool          *pgxpool.Pool
	Log           logrus.FieldLogger
	Agent         *AgentConfig
}

func Run(ctx context.Context, cfg *Config) error {
	cfg.Log.Info("GRPC serving on ", cfg.ListenAddress)
	lis, err := net.Listen("tcp", cfg.ListenAddress)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	opts := []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	}
	s := grpc.NewServer(opts...)

	protoapi.RegisterTeamsServer(s, grpcteam.NewServer(cfg.Pool))
	protoapi.RegisterUsersServer(s, grpcuser.NewServer(cfg.Pool))
	protoapi.RegisterReconcilersServer(s, grpcreconciler.NewServer(cfg.Pool))
	protoapi.RegisterDeploymentsServer(s, grpcdeployment.NewServer(cfg.Pool))

	// Register agent server if configured
	if cfg.Agent != nil {
		protoapi.RegisterAgentServer(s, grpcagent.NewServer(
			cfg.Pool,
			cfg.Agent.ChatClient,
			cfg.Agent.RAGClient,
			cfg.Agent.NaisAPIURL,
			cfg.Log.WithField("service", "agent"),
		))
		cfg.Log.Info("Agent gRPC service registered")
	}

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return s.Serve(lis) })
	g.Go(func() error {
		<-ctx.Done()

		ch := make(chan struct{})
		go func() {
			s.GracefulStop()
			close(ch)
		}()

		select {
		case <-ch:
			// ok
		case <-time.After(5 * time.Second):
			// force shutdown
			s.Stop()
		}

		return nil
	})

	return g.Wait()
}

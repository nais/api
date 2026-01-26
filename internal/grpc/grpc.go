package grpc

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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

// Config holds configuration for the gRPC server.
type Config struct {
	ListenAddress string
	Pool          *pgxpool.Pool
	Log           logrus.FieldLogger
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

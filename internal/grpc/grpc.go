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

func Run(ctx context.Context, listenAddress string, pool *pgxpool.Pool, log logrus.FieldLogger) error {
	log.Info("GRPC serving on ", listenAddress)
	lis, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	opts := []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	}
	s := grpc.NewServer(opts...)

	protoapi.RegisterTeamsServer(s, grpcteam.NewServer(pool))
	protoapi.RegisterUsersServer(s, grpcuser.NewServer(pool))
	protoapi.RegisterReconcilersServer(s, grpcreconciler.NewServer(pool))
	protoapi.RegisterDeploymentsServer(s, grpcdeployment.NewServer(pool))

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

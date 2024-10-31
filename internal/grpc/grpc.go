package grpc

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/nais/api/internal/grpc/grpcuser"
	"github.com/nais/api/internal/grpc/grpcuser/grpcusersql"

	"github.com/nais/api/internal/auditlogger"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/grpc/grpcteam"
	"github.com/nais/api/internal/grpc/grpcteam/grpcteamsql"
	"github.com/nais/api/pkg/apiclient/protoapi"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

func Run(ctx context.Context, listenAddress string, repo database.Database, auditlog auditlogger.AuditLogger, log logrus.FieldLogger) error {
	log.Info("GRPC serving on ", listenAddress)
	lis, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	opts := []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	}
	s := grpc.NewServer(opts...)

	pool := repo.GetPool()
	protoapi.RegisterTeamsServer(s, grpcteam.NewServer(grpcteamsql.New(pool)))
	protoapi.RegisterUsersServer(s, grpcuser.NewServer(grpcusersql.New(pool)))
	protoapi.RegisterReconcilersServer(s, &ReconcilersServer{db: repo})
	protoapi.RegisterAuditLogsServer(s, &AuditLogsServer{db: repo, auditlog: auditlog})

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

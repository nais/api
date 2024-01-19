package grpc

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/nais/api/internal/database"
	"github.com/nais/api/pkg/protoapi"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

func Run(ctx context.Context, listenAddress string, repo database.Database, log logrus.FieldLogger) error {
	log.Info("GRPC serving on ", listenAddress)
	lis, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	opts := []grpc.ServerOption{}
	s := grpc.NewServer(opts...)

	protoapi.RegisterTeamsServer(s, &TeamsServer{db: repo})
	protoapi.RegisterUsersServer(s, &UsersServer{db: repo})
	protoapi.RegisterReconcilerResourcesServer(s, &ReconcilerResourcesServer{db: repo})

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

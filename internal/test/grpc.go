package test

import (
	"context"
	"net"

	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type GRPCServer struct {
	conn *bufconn.Listener
	srv  *grpc.Server
}

type RegisterServiceFuncs func(*grpc.Server)

func StartGRPCServer(services ...RegisterServiceFuncs) *GRPCServer {
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	for _, regFunc := range services {
		regFunc(s)
	}

	f := &GRPCServer{lis, s}

	go func() {
		if err := f.srv.Serve(f.conn); err != nil {
			logrus.Errorf("failed to start Fake GRPC server: %v", err)
		}
	}()
	return f
}

func (f *GRPCServer) Close() {
	f.srv.Stop()
	err := f.conn.Close()
	if err != nil {
		logrus.Errorf("failed to close bufconn: %v", err)
		return
	}
}

func (f *GRPCServer) ClientOptions() []option.ClientOption {
	return []option.ClientOption{
		option.WithGRPCDialOption(grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return f.conn.Dial()
		})),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
		option.WithGRPCDialOption(grpc.WithBlock()),
		option.WithoutAuthentication(),
	}
}

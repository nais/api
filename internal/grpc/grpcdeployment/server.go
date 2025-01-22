package grpcdeployment

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/grpc/grpcdeployment/grpcdeploymentsql"
	"github.com/nais/api/pkg/apiclient/protoapi"
)

type Server struct {
	querier grpcdeploymentsql.Querier
	protoapi.UnimplementedDeploymentsServer
}

func NewServer(pool *pgxpool.Pool) *Server {
	return &Server{
		querier: grpcdeploymentsql.New(pool),
	}
}

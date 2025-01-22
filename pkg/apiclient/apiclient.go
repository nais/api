package apiclient

import (
	"github.com/nais/api/pkg/apiclient/protoapi"
	"google.golang.org/grpc"
)

type APIClient struct {
	conn *grpc.ClientConn
}

func New(target string, opts ...grpc.DialOption) (*APIClient, error) {
	gclient, err := grpc.NewClient(target, opts...)
	if err != nil {
		panic("Failed to connect to provider " + err.Error())
	}

	return &APIClient{
		conn: gclient,
	}, nil
}

func (a *APIClient) Reconcilers() protoapi.ReconcilersClient {
	return protoapi.NewReconcilersClient(a.conn)
}

func (a *APIClient) Users() protoapi.UsersClient {
	return protoapi.NewUsersClient(a.conn)
}

func (a *APIClient) Teams() protoapi.TeamsClient {
	return protoapi.NewTeamsClient(a.conn)
}

func (a *APIClient) Deployments() protoapi.DeploymentsClient {
	return protoapi.NewDeploymentsClient(a.conn)
}

func (a *APIClient) Close() error {
	return a.conn.Close()
}

package main

import (
	"context"
	"fmt"

	"github.com/nais/api/pkg/apiclient"
	"github.com/nais/api/pkg/protoapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	ctx := context.Background()
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	client, err := apiclient.New("127.0.0.1:3001", opts...)
	if err != nil {
		panic(err)
	}

	teams, err := client.Users().List(ctx, &protoapi.ListUsersRequest{
		Limit:  10,
		Offset: 3,
	})
	if err != nil {
		panic(err)
	}

	for _, team := range teams.Nodes {
		fmt.Println(team.Name)
	}
}

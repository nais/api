package main

import (
	"context"
	"fmt"

	"github.com/nais/api/pkg/protoapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	ctx := context.Background()
	var opts []grpc.DialOption

	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	gclient, err := grpc.Dial("127.0.0.1:3001", opts...)
	if err != nil {
		panic("Failed to connect to provider " + err.Error())
	}

	client := protoapi.NewUsersClient(gclient)
	teams, err := client.List(ctx, &protoapi.ListUsersRequest{
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

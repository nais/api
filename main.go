package main

import (
	"context"
	"fmt"

	"github.com/nais/api/pkg/apiclient"
	"github.com/nais/api/pkg/apiclient/iterator"
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

	it := iterator.New(ctx, 2, func(limit, offset int64) (*protoapi.ListTeamsResponse, error) {
		return client.Teams().List(ctx, &protoapi.ListTeamsRequest{
			Limit:  limit,
			Offset: offset,
		})
	})

	count := 0
	for it.Next() {
		fmt.Printf("%+v\n", it.Value().Slug)
		count += 1
	}

	if err := it.Err(); err != nil {
		panic(err)
	}
	fmt.Println("count:", count)
}

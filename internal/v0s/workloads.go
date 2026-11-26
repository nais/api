package v0s

import (
	"context"
	"fmt"

	protoapi "github.com/nais/v0s/api/v0s/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type WorkloadClient interface {
	ListWorkloads(context.Context, *protoapi.ListWorkloadsRequest, ...grpc.CallOption) (*protoapi.ListWorkloadsResponse, error)
}

func ListWorkloads(ctx context.Context, client WorkloadClient, input *protoapi.ListWorkloadsRequest) ([]*protoapi.Workload, error) {
	req := proto.Clone(input).(*protoapi.ListWorkloadsRequest)
	var result []*protoapi.Workload
	seenTokens := map[string]bool{req.PageToken: true}
	for {
		resp, err := client.ListWorkloads(ctx, req)
		if err != nil {
			return nil, backendError("list v0s workloads", err)
		}
		if resp == nil {
			return nil, fmt.Errorf("v0s returned no workload response")
		}
		for _, row := range resp.GetWorkloads() {
			if row == nil || row.GetEnvironment() == "" || row.GetTeam() == "" || row.GetName() == "" {
				return nil, fmt.Errorf("v0s returned a workload without an identity")
			}
			result = append(result, row)
		}
		next := resp.GetNextPageToken()
		if next == "" {
			return result, nil
		}
		if seenTokens[next] {
			return nil, fmt.Errorf("v0s returned a non-advancing workload cursor")
		}
		seenTokens[next] = true
		req.PageToken = next
	}
}

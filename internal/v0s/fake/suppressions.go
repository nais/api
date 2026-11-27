package fake

import (
	"context"
	"slices"
	"strings"

	protoapi "github.com/nais/v0s/api/v0s/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type suppressionKey struct {
	team, environment, workload, image, pkg, vulnerability string
}

func findingSuppressionKey(f *protoapi.Finding) suppressionKey {
	return suppressionKey{f.Workload.Team, f.Workload.Environment, f.Workload.Name, f.Image.Name, f.PackageKey, f.VulnerabilityId}
}

func suppressionID(s *protoapi.Suppression) suppressionKey {
	return suppressionKey{s.Team, s.Environment, s.Workload, s.ImageName, s.PackageKey, s.VulnerabilityId}
}

func (c *Client) suppression(ctx context.Context, team string, key *protoapi.SuppressionKey) (*protoapi.Suppression, error) {
	if team == "" || key.GetEnvironment() == "" || key.GetWorkload() == "" || key.GetImageName() == "" || key.GetPackage() == "" {
		return nil, status.Error(codes.InvalidArgument, "all suppression key fields are required")
	}
	v, err := c.GetVulnerability(ctx, &protoapi.GetVulnerabilityRequest{Id: key.GetVulnerabilityId()})
	if err != nil {
		return nil, err
	}
	pkg := key.Package
	if i := strings.IndexAny(pkg, "@?#"); i >= 0 {
		pkg = pkg[:i]
	}
	return &protoapi.Suppression{
		Team: team, Environment: key.Environment, Workload: key.Workload,
		ImageName: key.ImageName, PackageKey: pkg, VulnerabilityId: v.Vulnerability.Id,
	}, nil
}

func (c *Client) SetSuppressions(ctx context.Context, req *protoapi.SetSuppressionsRequest, _ ...grpc.CallOption) (*protoapi.SetSuppressionsResponse, error) {
	resp := &protoapi.SetSuppressionsResponse{}
	for _, input := range req.Suppressions {
		s, err := c.suppression(ctx, req.Team, input.GetKey())
		if err != nil {
			return nil, err
		}
		switch input.State {
		case protoapi.SuppressionState_SUPPRESSION_STATE_NOT_AFFECTED,
			protoapi.SuppressionState_SUPPRESSION_STATE_FALSE_POSITIVE,
			protoapi.SuppressionState_SUPPRESSION_STATE_ACCEPTED_RISK:
		default:
			return nil, status.Error(codes.InvalidArgument, "invalid suppression state")
		}
		s.State, s.Detail, s.UpdatedBy, s.UpdatedAt = input.State, input.Detail, req.Actor, timestamppb.Now()
		resp.Suppressions = append(resp.Suppressions, s)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, s := range resp.Suppressions {
		c.suppressions[suppressionID(s)] = clone(s)
	}
	return resp, nil
}

func (c *Client) DeleteSuppressions(ctx context.Context, req *protoapi.DeleteSuppressionsRequest, _ ...grpc.CallOption) (*protoapi.DeleteSuppressionsResponse, error) {
	keys := make([]suppressionKey, 0, len(req.Keys))
	for _, key := range req.Keys {
		s, err := c.suppression(ctx, req.Team, key)
		if err != nil {
			return nil, err
		}
		keys = append(keys, suppressionID(s))
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	resp := &protoapi.DeleteSuppressionsResponse{}
	for _, key := range keys {
		if s := c.suppressions[key]; s != nil {
			resp.Deleted = append(resp.Deleted, clone(s))
			delete(c.suppressions, key)
		}
	}
	return resp, nil
}

func (c *Client) ListSuppressions(ctx context.Context, req *protoapi.ListSuppressionsRequest, _ ...grpc.CallOption) (*protoapi.ListSuppressionsResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.RLock()
	var rows []*protoapi.Suppression
	for _, s := range c.suppressions {
		if matchesScope(s.Environment, s.Team, s.Workload, req.Environment, req.Team, req.Workload) &&
			(req.ImageName == "" || req.ImageName == s.ImageName) &&
			(req.VulnerabilityId == "" || matchesVulnerability(s.VulnerabilityId, req.VulnerabilityId)) {
			rows = append(rows, clone(s))
		}
	}
	c.mu.RUnlock()
	slices.SortFunc(rows, func(a, b *protoapi.Suppression) int {
		return strings.Compare(
			strings.Join([]string{a.Team, a.Environment, a.Workload, a.ImageName, a.PackageKey, a.VulnerabilityId}, "/"),
			strings.Join([]string{b.Team, b.Environment, b.Workload, b.ImageName, b.PackageKey, b.VulnerabilityId}, "/"),
		)
	})
	rows, cursors, next, info, err := paginate(rows, req.PageSize, req.PageToken, req.PageDirection)
	return &protoapi.ListSuppressionsResponse{Suppressions: rows, Cursors: cursors, NextPageToken: next, PageInfo: info}, err
}

func (*Client) ForwardV13SSuppression(context.Context, *protoapi.ForwardV13SSuppressionRequest, ...grpc.CallOption) (*protoapi.ForwardV13SSuppressionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "v13s synchronization is not used by the local fake")
}

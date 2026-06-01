package graph

import (
	"context"
	"errors"

	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/tunnel"
)

func (r *mutationResolver) CreateTunnel(ctx context.Context, input tunnel.CreateTunnelInput) (*tunnel.CreateTunnelPayload, error) {
	return tunnel.Create(ctx, input)
}

func (r *mutationResolver) DeleteTunnel(ctx context.Context, input tunnel.DeleteTunnelInput) (*tunnel.DeleteTunnelPayload, error) {
	if err := tunnel.Delete(ctx, input.TeamSlug, input.EnvironmentName, input.TunnelName); err != nil {
		return nil, err
	}
	return &tunnel.DeleteTunnelPayload{Success: true}, nil
}

func (r *teamEnvironmentResolver) Tunnel(ctx context.Context, obj *team.TeamEnvironment, name string) (*tunnel.Tunnel, error) {
	t, err := tunnel.Get(ctx, obj.TeamSlug.String(), obj.EnvironmentName, name)
	if errors.Is(err, tunnel.ErrTunnelNotFound) {
		return nil, nil
	}
	return t, err
}

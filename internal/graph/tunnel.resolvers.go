package graph

import (
	"context"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/model/donotuse"
	"github.com/nais/api/internal/slug"
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
	return tunnel.Get(ctx, obj.TeamSlug.String(), obj.EnvironmentName, name)
}

func (r *tunnelResolver) Phase(ctx context.Context, obj *tunnel.Tunnel) (donotuse.TunnelPhase, error) {
	return donotuse.TunnelPhase(obj.Phase), nil
}

func (r *tunnelResolver) Target(ctx context.Context, obj *tunnel.Tunnel) (*donotuse.TunnelTarget, error) {
	return &donotuse.TunnelTarget{
		Host: obj.Target.Host,
		Port: int(obj.Target.Port),
	}, nil
}

func (r *createTunnelInputResolver) TeamSlug(ctx context.Context, obj *tunnel.CreateTunnelInput, data slug.Slug) error {
	obj.TeamSlug = data.String()
	return nil
}

func (r *deleteTunnelInputResolver) TeamSlug(ctx context.Context, obj *tunnel.DeleteTunnelInput, data slug.Slug) error {
	obj.TeamSlug = data.String()
	return nil
}

func (r *Resolver) Tunnel() gengql.TunnelResolver { return &tunnelResolver{r} }

func (r *Resolver) CreateTunnelInput() gengql.CreateTunnelInputResolver {
	return &createTunnelInputResolver{r}
}

func (r *Resolver) DeleteTunnelInput() gengql.DeleteTunnelInputResolver {
	return &deleteTunnelInputResolver{r}
}

type (
	tunnelResolver            struct{ *Resolver }
	createTunnelInputResolver struct{ *Resolver }
	deleteTunnelInputResolver struct{ *Resolver }
)

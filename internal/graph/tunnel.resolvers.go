package graph

import (
	"context"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model/donotuse"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/tunnel"
)

func (r *mutationResolver) CreateTunnel(ctx context.Context, input tunnel.CreateTunnelInput) (*tunnel.CreateTunnelPayload, error) {
	return tunnel.Create(ctx, input)
}

func (r *mutationResolver) UpdateTunnelSTUNEndpoint(ctx context.Context, input tunnel.UpdateTunnelSTUNEndpointInput) (*tunnel.UpdateTunnelSTUNEndpointPayload, error) {
	return tunnel.UpdateSTUNEndpoint(ctx, input.TunnelID, input.ClientSTUNEndpoint)
}

func (r *mutationResolver) DeleteTunnel(ctx context.Context, input tunnel.DeleteTunnelInput) (*tunnel.DeleteTunnelPayload, error) {
	if err := tunnel.Delete(ctx, input.TunnelID); err != nil {
		return nil, err
	}
	return &tunnel.DeleteTunnelPayload{Success: true}, nil
}

func (r *teamEnvironmentResolver) Tunnel(ctx context.Context, obj *team.TeamEnvironment, id ident.Ident) (*tunnel.Tunnel, error) {
	return tunnel.Get(ctx, id.ID)
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

func (r *tunnelCreatedActivityLogEntryResolver) TunnelID(ctx context.Context, obj *tunnel.TunnelCreatedActivityLogEntry) (*ident.Ident, error) {
	return &ident.Ident{ID: obj.TunnelID, Type: "Tunnel"}, nil
}

func (r *tunnelDeletedActivityLogEntryResolver) TunnelID(ctx context.Context, obj *tunnel.TunnelDeletedActivityLogEntry) (*ident.Ident, error) {
	return &ident.Ident{ID: obj.TunnelID, Type: "Tunnel"}, nil
}

func (r *createTunnelInputResolver) TeamSlug(ctx context.Context, obj *tunnel.CreateTunnelInput, data slug.Slug) error {
	obj.TeamSlug = data.String()
	return nil
}

func (r *deleteTunnelInputResolver) TunnelID(ctx context.Context, obj *tunnel.DeleteTunnelInput, data *ident.Ident) error {
	if data != nil {
		obj.TunnelID = data.ID
	}
	return nil
}

func (r *updateTunnelSTUNEndpointInputResolver) TunnelID(ctx context.Context, obj *tunnel.UpdateTunnelSTUNEndpointInput, data *ident.Ident) error {
	if data != nil {
		obj.TunnelID = data.ID
	}
	return nil
}

func (r *Resolver) Tunnel() gengql.TunnelResolver { return &tunnelResolver{r} }

func (r *Resolver) TunnelCreatedActivityLogEntry() gengql.TunnelCreatedActivityLogEntryResolver {
	return &tunnelCreatedActivityLogEntryResolver{r}
}

func (r *Resolver) TunnelDeletedActivityLogEntry() gengql.TunnelDeletedActivityLogEntryResolver {
	return &tunnelDeletedActivityLogEntryResolver{r}
}

func (r *Resolver) CreateTunnelInput() gengql.CreateTunnelInputResolver {
	return &createTunnelInputResolver{r}
}

func (r *Resolver) DeleteTunnelInput() gengql.DeleteTunnelInputResolver {
	return &deleteTunnelInputResolver{r}
}

func (r *Resolver) UpdateTunnelSTUNEndpointInput() gengql.UpdateTunnelSTUNEndpointInputResolver {
	return &updateTunnelSTUNEndpointInputResolver{r}
}

type (
	tunnelResolver                        struct{ *Resolver }
	tunnelCreatedActivityLogEntryResolver struct{ *Resolver }
	tunnelDeletedActivityLogEntryResolver struct{ *Resolver }
	createTunnelInputResolver             struct{ *Resolver }
	deleteTunnelInputResolver             struct{ *Resolver }
	updateTunnelSTUNEndpointInputResolver struct{ *Resolver }
)

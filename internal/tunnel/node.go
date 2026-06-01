package tunnel

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
)

type identType int

const (
	identTunnel identType = iota
)

func init() {
	ident.RegisterIdentType(identTunnel, "TU", GetByIdent)
}

func newTunnelIdent(teamSlug slug.Slug, environment, name string) ident.Ident {
	return ident.NewIdent(identTunnel, teamSlug.String(), environment, name)
}

func parseTunnelIdent(id ident.Ident) (teamSlug slug.Slug, environment, name string, err error) {
	parts := id.Parts()
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid tunnel ident")
	}

	return slug.Slug(parts[0]), parts[1], parts[2], nil
}

func GetByIdent(ctx context.Context, id ident.Ident) (*Tunnel, error) {
	teamSlug, environment, name, err := parseTunnelIdent(id)
	if err != nil {
		return nil, err
	}

	loaders := FromContext(ctx)
	if loaders == nil {
		return nil, fmt.Errorf("tunnel loaders not found in context")
	}

	for _, w := range loaders.tunnelWatcher.All() {
		if w.Obj.TeamSlug == teamSlug.String() && w.Obj.Environment == environment && w.Obj.Name == name {
			return w.Obj, nil
		}
	}

	return nil, ErrTunnelNotFound
}

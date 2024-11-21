package team

import (
	"fmt"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
)

// identType is an enumeration type for the different identifiers defined by this domain package. Each domain package
// must have its own enumeration type to not cause collisions with other domain packages.
type identType int

const (
	// A list of identifiers defined by this package.

	identTeam identType = iota
	identTeamEnvironment
)

func init() {
	// Register all identifiers during initialization. The first argument is the constant identifier type, the second is
	// a globally unique string representation of the identifier type, and should be as short as possible. The third
	// argument is a lookup function that can be used to retrieve the node associated with the identifier. The return
	// type of the lookup function must be compatible with the model.Node interface.
	//
	// Refer to https://go.dev/doc/effective_go#init for more information about the init() function itself.

	ident.RegisterIdentType(identTeam, "T", GetByIdent)
	ident.RegisterIdentType(identTeamEnvironment, "TE", GetTeamEnvironmentByIdent)
}

// newTeamIdent creates a new identifier for a specific team
func newTeamIdent(slug slug.Slug) ident.Ident {
	return ident.NewIdent(identTeam, slug.String())
}

func newTeamEnvironmentIdent(slug slug.Slug, envName string) ident.Ident {
	return ident.NewIdent(identTeamEnvironment, slug.String(), envName)
}

// parseTeamIdent returns the team slug from a team identifier. If the identifier is invalid, an error is returned.
func parseTeamIdent(id ident.Ident) (slug.Slug, error) {
	parts := id.Parts()
	if len(parts) != 1 {
		return "", fmt.Errorf("invalid team ident")
	}

	return slug.Slug(parts[0]), nil
}

func parseTeamEnvironmentIdent(id ident.Ident) (slug.Slug, string, error) {
	parts := id.Parts()
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid team environment ident")
	}

	return slug.Slug(parts[0]), parts[1], nil
}

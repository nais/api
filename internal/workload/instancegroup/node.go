package instancegroup

import (
	"fmt"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
)

type identType int

const (
	identKey    identType = iota
	identKeyEnv identType = iota
)

func init() {
	ident.RegisterIdentType(identKey, "IG", GetByIdent)
	ident.RegisterIdentType(identKeyEnv, "IGEV", GetEnvironmentVariableByIdent)
}

func newIdent(teamSlug slug.Slug, environment, applicationName, instanceGroupName string) ident.Ident {
	return ident.NewIdent(identKey, teamSlug.String(), environment, applicationName, instanceGroupName)
}

func newEnvVarIdent(teamSlug slug.Slug, environment, applicationName, instanceGroupName, envVarName string) ident.Ident {
	return ident.NewIdent(identKeyEnv, teamSlug.String(), environment, applicationName, instanceGroupName, envVarName)
}

func parseIdent(id ident.Ident) (teamSlug slug.Slug, environment, applicationName, instanceGroupName string, err error) {
	parts := id.Parts()
	if len(parts) != 4 {
		return "", "", "", "", fmt.Errorf("invalid instance group ident")
	}

	return slug.Slug(parts[0]), parts[1], parts[2], parts[3], nil
}

func parseEnvVarIdent(id ident.Ident) (teamSlug slug.Slug, environment, applicationName, instanceGroupName, envVarName string, err error) {
	parts := id.Parts()
	if len(parts) != 5 {
		return "", "", "", "", "", fmt.Errorf("invalid instance group environment variable ident")
	}

	return slug.Slug(parts[0]), parts[1], parts[2], parts[3], parts[4], nil
}

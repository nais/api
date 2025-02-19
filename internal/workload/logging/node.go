package logging

import (
	"fmt"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
)

type identType int

const (
	identKey identType = iota
)

func init() {
	ident.RegisterIdentType(identKey, "LD", getByIdent)
}

func newIdent(kind SupportedLogDestination, workloadType workload.Type, teamSlug slug.Slug, environment, name string) ident.Ident {
	return ident.NewIdent(identKey, kind.String(), workloadType.String(), teamSlug.String(), environment, name)
}

func parseIdent(id ident.Ident) (kind SupportedLogDestination, workloadType workload.Type, teamSlug slug.Slug, environment, name string, err error) {
	parts := id.Parts()
	if len(parts) != 5 {
		return "", 0, "", "", "", fmt.Errorf("invalid log destination ident")
	}

	workloadType, err = workload.TypeFromString(parts[1])
	if err != nil {
		return "", 0, "", "", "", err
	}

	return SupportedLogDestination(parts[0]), workloadType, slug.Slug(parts[2]), parts[3], parts[4], nil
}

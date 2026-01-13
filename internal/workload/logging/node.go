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

func newGenericIdent(kind SupportedLogDestination, workloadType workload.Type, teamSlug slug.Slug, environment, workloadName, logName string) ident.Ident {
	return ident.NewIdent(identKey, kind.String(), workloadType.String(), teamSlug.String(), environment, workloadName, logName)
}

// parseIdent parses a log destination ident into its components. LogName is optional and only present for generic log destinations.
func parseIdent(id ident.Ident) (kind SupportedLogDestination, workloadType workload.Type, teamSlug slug.Slug, environment, workloadName, logName string, err error) {
	parts := id.Parts()
	if len(parts) == 6 {
		workloadType, err = workload.TypeFromString(parts[1])
		if err != nil {
			return "", 0, "", "", "", "", err
		}

		return SupportedLogDestination(parts[0]), workloadType, slug.Slug(parts[2]), parts[3], parts[4], parts[5], nil
	}

	if len(parts) != 5 {
		return "", 0, "", "", "", "", fmt.Errorf("invalid log destination ident")
	}

	workloadType, err = workload.TypeFromString(parts[1])
	if err != nil {
		return "", 0, "", "", "", "", err
	}

	return SupportedLogDestination(parts[0]), workloadType, slug.Slug(parts[2]), parts[3], parts[4], "", nil
}

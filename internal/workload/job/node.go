package job

import (
	"fmt"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
)

type identType int

const (
	identKey identType = iota
	jobRunIdentKey
	jobRunInstanceIdentKey
)

func init() {
	ident.RegisterIdentType(identKey, "J", GetByIdent)
	ident.RegisterIdentType(jobRunIdentKey, "JR", GetByJobRunIdent)
	ident.RegisterIdentType(jobRunInstanceIdentKey, "JRI", getJobRunInstanceByIdent)
}

func newIdent(teamSlug slug.Slug, environment, name string) ident.Ident {
	return ident.NewIdent(identKey, teamSlug.String(), environment, name)
}

func newJobRunIdent(teamSlug slug.Slug, environment, name string) ident.Ident {
	return ident.NewIdent(jobRunIdentKey, teamSlug.String(), environment, name)
}

func newJobRunInstanceIdent(teamSlug slug.Slug, environment, name string) ident.Ident {
	return ident.NewIdent(jobRunInstanceIdentKey, teamSlug.String(), environment, name)
}

func parseIdent(id ident.Ident) (teamSlug slug.Slug, environment, name string, err error) {
	parts := id.Parts()
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid job ident")
	}

	return slug.Slug(parts[0]), parts[1], parts[2], nil
}

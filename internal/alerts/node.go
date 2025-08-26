package alerts

import (
	"fmt"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
)

type identType int

const (
	identKey identType = iota
)

func init() {
	ident.RegisterIdentType(identKey, "ALRT", GetByIdent)
}

func newIdent(alertType AlertType, teamSlug slug.Slug, environment, ruleGroup, alertName string) ident.Ident {
	return ident.NewIdent(identKey, alertType.String(), teamSlug.String(), environment, ruleGroup, alertName)
}

func parseIdent(id ident.Ident) (alertType AlertType, teamSlug slug.Slug, environment, ruleGroup, alertName string, err error) {
	parts := id.Parts()
	if len(parts) != 5 {
		return -1, "", "", "", "", fmt.Errorf("invalid alert ident")
	}

	alertType, err = AlertTypeFromString(parts[0])
	if err != nil {
		return -1, "", "", "", "", fmt.Errorf("invalid alert ident: %w", err)
	}

	return alertType, slug.Slug(parts[1]), parts[2], parts[3], parts[4], nil
}

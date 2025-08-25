package issuechecker

import (
	"context"

	"github.com/nais/api/internal/graph/ident"
)

func GetByIdent(ctx context.Context, id ident.Ident) (*Issue, error) {
	return nil, nil
}

func GetIssues(ctx context.Context, teamSlug string) ([]*Issue, error) {
	return []*Issue{
		{
			ID:           newIdent("balls"),
			ResourceName: "string",
			Details: AivenAlertDetails{
				ID:      newIdent("bb"),
				Message: "string",
			},
		},
		{
			ID:           newIdent("balls2"),
			ResourceName: "string2",
			Details: AivenAlertDetails{
				ID:      newIdent("bb2"),
				Message: "stringxxxx",
			},
		},
	}, nil
}

package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/issue"
)

func (r *sQLInstanceIssueResolver) State(ctx context.Context, obj *issue.SQLInstanceIssue) (issue.SQLInstanceIssueState, error) {
	panic(fmt.Errorf("not implemented: State - state"))
}

func (r *Resolver) SQLInstanceIssue() gengql.SQLInstanceIssueResolver {
	return &sQLInstanceIssueResolver{r}
}

type sQLInstanceIssueResolver struct{ *Resolver }

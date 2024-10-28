package application

import (
	"context"
	"strings"

	"github.com/nais/api/internal/v1/graphv1/sortfilter"
)

var SortFilter = sortfilter.New[*Application, ApplicationOrderField, struct{}](ApplicationOrderFieldName)

func init() {
	SortFilter.RegisterOrderBy(ApplicationOrderFieldName, func(ctx context.Context, a, b *Application) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	SortFilter.RegisterOrderBy(ApplicationOrderFieldEnvironment, func(ctx context.Context, a, b *Application) int {
		return strings.Compare(a.GetEnvironmentName(), b.GetEnvironmentName())
	})
}

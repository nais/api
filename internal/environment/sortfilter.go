package environment

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/sortfilter"
)

var SortFilter = sortfilter.New[*Environment, EnvironmentOrderField, *struct{}]("NAME", model.OrderDirectionAsc)

func init() {
	SortFilter.RegisterSort("NAME", func(ctx context.Context, a, b *Environment) int {
		return strings.Compare(a.Name, b.Name)
	})
}

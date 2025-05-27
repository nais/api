package team

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
)

var SortFilter = sortfilter.New[*Team, TeamOrderField, struct{}]()

func init() {
	SortFilter.RegisterSort("_SLUG", func(ctx context.Context, a, b *Team) int {
		return strings.Compare(a.Slug.String(), b.Slug.String())
	})
}

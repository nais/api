package postgres

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/sortfilter"
)

func (f *PostgresFilter) Matches(pg *Postgres) bool {
	return f == nil || (len(f.Environments) == 0 || slices.Contains(f.Environments, pg.EnvironmentName)) && model.MatchesLabelFilters(pg.Labels, f.Labels)
}

var SortFilterPostgresBranch = sortfilter.New[*PostgresBranch, PostgresBranchOrderField, *PostgresBranchFilter]()

func init() {
	SortFilterPostgresBranch.RegisterSort("NAME", func(ctx context.Context, a, b *PostgresBranch) int {
		if a.Name != b.Name {
			return strings.Compare(a.Name, b.Name)
		}
		return strings.Compare(a.PostgresName, b.PostgresName)
	}, "ENVIRONMENT")
	SortFilterPostgresBranch.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b *PostgresBranch) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	}, "NAME")

	SortFilterPostgresBranch.RegisterFilter(func(ctx context.Context, v *PostgresBranch, filter *PostgresBranchFilter) bool {
		if filter.Name != "" {
			if !strings.Contains(strings.ToLower(v.Name), strings.ToLower(filter.Name)) {
				return false
			}
		}

		if len(filter.Environments) > 0 {
			if !slices.Contains(filter.Environments, v.EnvironmentName) {
				return false
			}
		}

		if len(filter.States) > 0 {
			if !slices.Contains(filter.States, v.State) {
				return false
			}
		}

		if !model.MatchesLabelFilters(v.Labels, filter.Labels) {
			return false
		}

		return true
	})
}

package postgres

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/sortfilter"
)

var SortFilterPostgresInstance = sortfilter.New[*PostgresInstance, PostgresInstanceOrderField, *PostgresInstanceFilter]()

func init() {
	SortFilterPostgresInstance.RegisterSort("NAME", func(ctx context.Context, a, b *PostgresInstance) int {
		return strings.Compare(a.GetName(), b.GetName())
	}, "ENVIRONMENT")
	SortFilterPostgresInstance.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b *PostgresInstance) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	}, "NAME")

	SortFilterPostgresInstance.RegisterFilter(func(ctx context.Context, v *PostgresInstance, filter *PostgresInstanceFilter) bool {
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

		if filter.HighAvailability != nil {
			if v.HighAvailability != *filter.HighAvailability {
				return false
			}
		}

		if len(filter.MajorVersions) > 0 {
			if !slices.Contains(filter.MajorVersions, v.MajorVersion) {
				return false
			}
		}

		if !model.MatchesLabelFilters(v.Labels, filter.Labels) {
			return false
		}

		return true
	})
}

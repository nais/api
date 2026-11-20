package postgres

import (
	"slices"

	"github.com/nais/api/internal/graph/model"
)

func (f *TeamPostgresFilter) Matches(pg *Postgres) bool {
	return f == nil || (len(f.Environments) == 0 || slices.Contains(f.Environments, pg.EnvironmentName)) && model.MatchesLabelFilters(pg.Labels, f.Labels)
}

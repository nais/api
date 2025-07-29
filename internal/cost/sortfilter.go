package cost

import (
	"context"

	"github.com/nais/api/internal/team"
)

const (
	TeamOrderAccumulatedCost team.TeamOrderField = "ACCUMULATED_COST"
)

func init() {
	teamInit()
}

func teamInit() {
	team.AllTeamOrderFields = append(team.AllTeamOrderFields, TeamOrderAccumulatedCost)

	team.SortFilter.RegisterConcurrentSort(TeamOrderAccumulatedCost, func(ctx context.Context, a *team.Team) int {
		tc, err := MonthlySummaryForTeam(ctx, a.Slug)
		if err != nil || tc == nil {
			return -1
		}

		return int(tc.Sum() * 1000000) // multiply by 1000000 to preserve precision
	}, "_SLUG")
}

package cost

import (
	"context"

	"github.com/nais/api/internal/team"
)

const (
	TeamOrderSumCost team.TeamOrderField = "SUM_COST"
)

func init() {
	teamInit()
}

func teamInit() {
	team.AllTeamOrderFields = append(team.AllTeamOrderFields, TeamOrderSumCost)

	team.SortFilter.RegisterConcurrentSort(TeamOrderSumCost, func(ctx context.Context, a *team.Team) int {
		tc, err := MonthlySummaryForTeam(ctx, a.Slug)
		if err != nil || tc == nil {
			return -1
		}

		return int(tc.Sum() * 1000) // multiply by 1000 to preserve precision
	}, "_SLUG")
}

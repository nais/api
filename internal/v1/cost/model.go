package cost

import "github.com/nais/api/internal/v1/graphv1/scalar"

type CostDaily struct {
	// The total cost for the period.
	Sum float64 `json:"sum"`
	// The cost series.
	Entries []*CostDailyEntry `json:"entries"`
}

type CostDailyEntry struct {
	// Name of the service the cost is for.
	Service string `json:"service"`
	// The cost for the service.
	Sum float64 `json:"sum"`
	// The cost series.
	Series []*CostSeries `json:"series"`
}

type CostMonthly struct {
	// The total cost for the period.
	Sum float64 `json:"sum"`
	// The cost series.
	Series []*CostSeries `json:"series"`
}

type CostSeries struct {
	// Date for the cost entry.
	Date scalar.Date `json:"date"`
	// The cost in euros.
	Sum float64 `json:"sum"`
}

type TeamCost struct {
	// Get the daily cost for a team.
	Daily *CostDaily `json:"daily"`
	// Get the monthly cost for a team. Will include up to 12 months of data.
	Monthly *CostMonthly `json:"monthly"`
}

type WorkloadCost struct {
	// Get the daily cost for a workload.
	Daily *CostDaily `json:"daily"`
	// Get the monthly cost for a workload. Will include up to 12 months of data.
	Monthly *CostMonthly `json:"monthly"`
}

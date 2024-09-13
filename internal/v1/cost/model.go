package cost

import (
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/scalar"
)

type WorkloadCost struct {
	EnvironmentName string    `json:"-"`
	WorkloadName    string    `json:"-"`
	TeamSlug        slug.Slug `json:"-"`
}

type WorkloadCostSeries struct {
	Date     scalar.Date            `json:"date"`
	Services []*WorkloadCostService `json:"services"`
}

func (w *WorkloadCostSeries) Sum() float64 {
	sum := 0.0
	for _, service := range w.Services {
		sum += service.Cost
	}
	return sum
}

type WorkloadCostPeriod struct {
	Series []*WorkloadCostSeries `json:"series"`
}

func (w *WorkloadCostPeriod) Sum() float64 {
	sum := 0.0
	for _, period := range w.Series {
		sum += period.Sum()
	}
	return sum
}

type WorkloadCostService struct {
	Service string  `json:"service"`
	Cost    float64 `json:"cost"`
}

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

type TeamCost struct {
	TeamSlug slug.Slug `json:"-"`
}

type ServiceCostSeries struct {
	Date     scalar.Date         `json:"date"`
	Services []*ServiceCostPoint `json:"services"`
}

func (w *ServiceCostSeries) Sum() float64 {
	sum := 0.0
	for _, service := range w.Services {
		sum += service.Cost
	}
	return sum
}

type WorkloadCostPeriod struct {
	Series []*ServiceCostSeries `json:"series"`
}

func (w *WorkloadCostPeriod) Sum() float64 {
	sum := 0.0
	for _, period := range w.Series {
		sum += period.Sum()
	}
	return sum
}

type ServiceCostPoint struct {
	Service string  `json:"service"`
	Cost    float64 `json:"cost"`
}

type TeamCostPeriod struct {
	Series []*ServiceCostSeries `json:"series"`
}

func (w *TeamCostPeriod) Sum() float64 {
	sum := 0.0
	for _, period := range w.Series {
		sum += period.Sum()
	}
	return sum
}

type TeamCostMonthlySample struct {
	Date scalar.Date `json:"date"`
	Cost float64     `json:"cost"`
}

type TeamCostMonthlySummary struct {
	Series []*TeamCostMonthlySample `json:"series"`
}

func (t *TeamCostMonthlySummary) Sum() float64 {
	sum := 0.0
	for _, period := range t.Series {
		sum += period.Cost
	}
	return sum
}

type BigQueryDatasetCost struct {
	Sum float64 `json:"sum"`
}

type OpenSearchCost struct {
	Sum float64 `json:"sum"`
}

type RedisInstanceCost struct {
	Sum float64 `json:"sum"`
}

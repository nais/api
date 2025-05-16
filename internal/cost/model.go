package cost

import (
	"context"

	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/validate"
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
	Date     scalar.Date          `json:"date"`
	Services []*ServiceCostSample `json:"services"`
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

type ServiceCostSample struct {
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

type TenantCost struct {
	Series []*TenantCostMonthlySample `json:"series"`
	Sum    float64                    `json:"sum"`
}

type TenantCostMonthlySample struct {
	Date    scalar.Date `json:"date"`
	Cost    float64     `json:"cost"`
	Service string      `json:"service"`
}
type TenantCostMonthlySummary struct {
	Series []*TenantCostMonthlySample `json:"series"`
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

type ValkeyInstanceCost struct {
	Sum float64 `json:"sum"`
}

type TeamEnvironmentCost struct {
	TeamSlug        slug.Slug `json:"-"`
	EnvironmentName string    `json:"-"`
}

type WorkloadCostSample struct {
	Cost            float64   `json:"cost"`
	WorkloadName    string    `json:"workloadName"`
	TeamSlug        slug.Slug `json:"-"`
	EnvironmentName string    `json:"-"`
}

type TeamEnvironmentCostPeriod struct {
	Series []*WorkloadCostSeries `json:"series"`
}

type WorkloadCostSeries struct {
	Date      scalar.Date           `json:"date"`
	Workloads []*WorkloadCostSample `json:"workloads"`
}

func (w *WorkloadCostSeries) Sum() float64 {
	sum := 0.0
	for _, workload := range w.Workloads {
		sum += workload.Cost
	}
	return sum
}

func (w *TeamEnvironmentCostPeriod) Sum() float64 {
	sum := 0.0
	for _, period := range w.Series {
		sum += period.Sum()
	}
	return sum
}

type SQLInstanceCost struct {
	Sum float64 `json:"sum"`
}

type TeamCostDailyFilter struct {
	// Services to include in the summary.
	Services []string `json:"services,omitempty"`
}

func (i *TeamCostDailyFilter) Validate(ctx context.Context) error {
	if i == nil {
		return nil
	}

	verr := validate.New()

	return verr.NilIfEmpty()
}

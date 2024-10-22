package cost

import (
	"context"
	"math/rand/v2"
	"sort"
	"time"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/scalar"
)

type FakeClient struct {
	dailyForTeamCache       map[string]*TeamCostPeriod
	dailyForWorkloadCache   map[string]*WorkloadCostPeriod
	monthlyForServiceCache  map[string]float32
	monthlyForWorkloadCache map[string]*WorkloadCostPeriod
	monthlySummaryCache     map[slug.Slug]*TeamCostMonthlySummary
}

func NewFakeClient() *FakeClient {
	return &FakeClient{
		dailyForTeamCache:       make(map[string]*TeamCostPeriod),
		dailyForWorkloadCache:   make(map[string]*WorkloadCostPeriod),
		monthlyForServiceCache:  make(map[string]float32),
		monthlyForWorkloadCache: make(map[string]*WorkloadCostPeriod),
		monthlySummaryCache:     make(map[slug.Slug]*TeamCostMonthlySummary),
	}
}

func (c *FakeClient) DailyForWorkload(_ context.Context, teamSlug slug.Slug, environmentName, workloadName string, fromDate, toDate time.Time) (*WorkloadCostPeriod, error) {
	cc := teamSlug.String() + environmentName + workloadName + fromDate.String() + toDate.String()
	if cached, exists := c.dailyForWorkloadCache[cc]; exists {
		return cached, nil
	}

	numDays := int(toDate.Sub(fromDate).Hours()/24) + 1 // inclusive
	series := make([]*ServiceCostSeries, numDays)
	for i := range numDays {
		serviceNames := randomServices()
		services := make([]*ServiceCost, len(serviceNames))
		for j := range len(serviceNames) {
			services[j] = &ServiceCost{
				Service: serviceNames[j],
				Cost:    rand.Float64(),
			}
		}
		series[i] = &ServiceCostSeries{
			Date:     scalar.Date(fromDate.AddDate(0, 0, i)),
			Services: services,
		}
	}
	c.dailyForWorkloadCache[cc] = &WorkloadCostPeriod{
		Series: series,
	}
	return c.dailyForWorkloadCache[cc], nil
}

func (c *FakeClient) MonthlyForWorkload(_ context.Context, teamSlug slug.Slug, environmentName, workloadName string) (*WorkloadCostPeriod, error) {
	cc := teamSlug.String() + environmentName + workloadName
	if cached, exists := c.monthlyForWorkloadCache[cc]; exists {
		return cached, nil
	}

	series := make([]*ServiceCostSeries, 12)
	now := time.Now()
	currentMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	for i := range 12 {
		serviceNames := randomServices()
		services := make([]*ServiceCost, len(serviceNames))
		for j := range len(serviceNames) {
			services[j] = &ServiceCost{
				Service: serviceNames[j],
				Cost:    rand.Float64(),
			}
		}
		series[i] = &ServiceCostSeries{
			Date:     scalar.Date(currentMonth.AddDate(0, -i, 0)),
			Services: services,
		}
	}
	c.monthlyForWorkloadCache[cc] = &WorkloadCostPeriod{
		Series: series,
	}
	return c.monthlyForWorkloadCache[cc], nil
}

func (c *FakeClient) DailyForTeam(_ context.Context, teamSlug slug.Slug, fromDate, toDate time.Time) (*TeamCostPeriod, error) {
	cc := teamSlug.String() + fromDate.String() + toDate.String()
	if cached, exists := c.dailyForTeamCache[cc]; exists {
		return cached, nil
	}

	numDays := int(toDate.Sub(fromDate).Hours()/24) + 1 // inclusive
	series := make([]*ServiceCostSeries, numDays)
	for i := range numDays {
		serviceNames := randomServices()
		services := make([]*ServiceCost, len(serviceNames))
		for j := range len(serviceNames) {
			services[j] = &ServiceCost{
				Service: serviceNames[j],
				Cost:    rand.Float64(),
			}
		}
		series[i] = &ServiceCostSeries{
			Date:     scalar.Date(fromDate.AddDate(0, 0, i)),
			Services: services,
		}
	}
	c.dailyForTeamCache[cc] = &TeamCostPeriod{
		Series: series,
	}
	return c.dailyForTeamCache[cc], nil
}

func (c *FakeClient) MonthlySummaryForTeam(_ context.Context, teamSlug slug.Slug) (*TeamCostMonthlySummary, error) {
	if cached, exists := c.monthlySummaryCache[teamSlug]; exists {
		return cached, nil
	}
	numMonthsToReturn := rand.IntN(12)
	if numMonthsToReturn == 0 {
		return &TeamCostMonthlySummary{}, nil
	}

	today := time.Now()
	currentMonth := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	samples := []*TeamCostMonthlySample{
		{
			Date: scalar.Date(currentMonth),
			Cost: rand.Float64(),
		},
	}
	for i := 1; i <= numMonthsToReturn; i++ {
		prevMonth := samples[i-1].Date.Time()
		samples = append(samples, &TeamCostMonthlySample{
			Date: scalar.Date(prevMonth.AddDate(0, 0, -prevMonth.Day())),
			Cost: rand.Float64(),
		})
	}
	c.monthlySummaryCache[teamSlug] = &TeamCostMonthlySummary{
		Series: samples,
	}
	return c.monthlySummaryCache[teamSlug], nil
}

func (c *FakeClient) MonthlyForService(_ context.Context, teamSlug slug.Slug, environmentName, workloadName, costType string) (float32, error) {
	cc := teamSlug.String() + environmentName + workloadName + costType
	if cached, exists := c.monthlyForServiceCache[cc]; exists {
		return cached, nil
	}

	cost := rand.Float32()
	c.monthlyForServiceCache[cc] = cost
	return cost, nil
}

func randomServices() []string {
	all := []string{
		"BigQuery",
		"Cloud SQL",
		"Cloud Storage",
		"Compute Engine",
		"Redis",
		"OpenSearch",
	}
	num := len(all)
	rand.Shuffle(num, func(i, j int) {
		all[i], all[j] = all[j], all[i]
	})
	ret := all[:rand.IntN(num+1)]
	sort.Strings(ret)
	return ret
}

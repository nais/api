package cost

import (
	"context"
	"math/rand/v2"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

type FakeClient struct {
	dailyForTeamCache            map[string]*TeamCostPeriod
	dailyForWorkloadCache        map[string]*WorkloadCostPeriod
	dailyForTeamEnvironmentCache map[string]*TeamEnvironmentCostPeriod
	monthlyForServiceCache       map[string]float32
	monthlyForWorkloadCache      map[string]*WorkloadCostPeriod
	monthlySummaryTeamCache      map[slug.Slug]*TeamCostMonthlySummary
	monthlySummaryTenantCache    *TenantCostMonthlySummary
	lock                         sync.Mutex
}

func NewFakeClient() *FakeClient {
	return &FakeClient{
		dailyForTeamCache:            make(map[string]*TeamCostPeriod),
		dailyForWorkloadCache:        make(map[string]*WorkloadCostPeriod),
		dailyForTeamEnvironmentCache: make(map[string]*TeamEnvironmentCostPeriod),
		monthlyForServiceCache:       make(map[string]float32),
		monthlyForWorkloadCache:      make(map[string]*WorkloadCostPeriod),
		monthlySummaryTeamCache:      make(map[slug.Slug]*TeamCostMonthlySummary),
		monthlySummaryTenantCache:    &TenantCostMonthlySummary{},
	}
}

var startOfCost = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

func (c *FakeClient) DailyForWorkload(_ context.Context, teamSlug slug.Slug, environmentName, workloadName string, fromDate, toDate time.Time) (*WorkloadCostPeriod, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	cc := teamSlug.String() + environmentName + workloadName + fromDate.String() + toDate.String()
	if cached, exists := c.dailyForWorkloadCache[cc]; exists {
		return cached, nil
	}

	numDays := int(toDate.Sub(fromDate).Hours()/24) + 1 // inclusive
	series := make([]*ServiceCostSeries, numDays)
	for i := range numDays {
		series[i] = serviceCostSeries(fromDate.AddDate(0, 0, i))
	}
	c.dailyForWorkloadCache[cc] = &WorkloadCostPeriod{
		Series: series,
	}
	return c.dailyForWorkloadCache[cc], nil
}

func (c *FakeClient) MonthlyForWorkload(_ context.Context, teamSlug slug.Slug, environmentName, workloadName string) (*WorkloadCostPeriod, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	cc := teamSlug.String() + environmentName + workloadName
	if cached, exists := c.monthlyForWorkloadCache[cc]; exists {
		return cached, nil
	}

	series := make([]*ServiceCostSeries, 12)
	now := time.Now()
	currentMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	for i := range 12 {
		monthStart := currentMonth.AddDate(0, -i, 0)
		var date time.Time
		if i == 0 {
			// Use the current date - 2 days for the current month
			date = now.AddDate(0, 0, -2)
		} else {
			// Use the last day of the month
			date = monthStart.AddDate(0, 1, -1)
		}
		series[i] = serviceCostSeries(date)
	}
	c.monthlyForWorkloadCache[cc] = &WorkloadCostPeriod{
		Series: series,
	}
	return c.monthlyForWorkloadCache[cc], nil
}

func (c *FakeClient) DailyForTeamEnvironment(ctx context.Context, teamSlug slug.Slug, environmentName string, fromDate, toDate time.Time) (*TeamEnvironmentCostPeriod, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	cc := teamSlug.String() + environmentName + fromDate.String() + toDate.String()
	if cached, exists := c.dailyForTeamEnvironmentCache[cc]; exists {
		return cached, nil
	}

	numDays := int(toDate.Sub(fromDate).Hours()/24) + 1 // inclusive
	series := make([]*WorkloadCostSeries, numDays)
	for i := range numDays {
		series[i] = workloadCostSeries(ctx, fromDate.AddDate(0, 0, i), teamSlug, environmentName)
	}
	c.dailyForTeamEnvironmentCache[cc] = &TeamEnvironmentCostPeriod{
		Series: series,
	}
	return c.dailyForTeamEnvironmentCache[cc], nil
}

func (c *FakeClient) DailyForTeam(_ context.Context, teamSlug slug.Slug, fromDate, toDate time.Time, filter *TeamCostDailyFilter) (*TeamCostPeriod, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	cc := teamSlug.String() + fromDate.String() + toDate.String()
	if cached, exists := c.dailyForTeamCache[cc]; exists {
		return filterDailyCost(cached, filter), nil
	}

	numDays := int(toDate.Sub(fromDate).Hours()/24) + 1 // inclusive
	series := make([]*ServiceCostSeries, numDays)
	for i := range numDays {
		series[i] = serviceCostSeries(fromDate.AddDate(0, 0, i))
	}
	c.dailyForTeamCache[cc] = &TeamCostPeriod{
		Series: series,
	}
	return filterDailyCost(c.dailyForTeamCache[cc], filter), nil
}

func (c *FakeClient) MonthlySummaryForTeam(_ context.Context, teamSlug slug.Slug) (*TeamCostMonthlySummary, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if cached, exists := c.monthlySummaryTeamCache[teamSlug]; exists {
		return cached, nil
	}
	numMonthsToReturn := rand.IntN(12)
	if numMonthsToReturn == 0 {
		return &TeamCostMonthlySummary{}, nil
	}

	today := time.Now()
	currentMonth := time.Date(today.Year(), today.Month(), today.Day()-2, 0, 0, 0, 0, today.Location())
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
	c.monthlySummaryTeamCache[teamSlug] = &TeamCostMonthlySummary{
		Series: samples,
	}
	return c.monthlySummaryTeamCache[teamSlug], nil
}

func (c *FakeClient) MonthlySummaryForTenant(_ context.Context) (*TenantCostMonthlySummary, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.monthlySummaryTenantCache != nil {
		return c.monthlySummaryTenantCache, nil
	}
	numMonthsToReturn := rand.IntN(12)
	if numMonthsToReturn == 0 {
		return &TenantCostMonthlySummary{}, nil
	}

	today := time.Now()
	currentMonth := time.Date(today.Year(), today.Month(), today.Day()-2, 0, 0, 0, 0, today.Location())
	samples := []*TenantCostMonthlySample{
		{
			Date: scalar.Date(currentMonth),
			Cost: rand.Float64(),
		},
	}
	for i := 1; i <= numMonthsToReturn; i++ {
		prevMonth := samples[i-1].Date.Time()
		samples = append(samples, &TenantCostMonthlySample{
			Date:    scalar.Date(prevMonth.AddDate(0, 0, -prevMonth.Day())),
			Cost:    rand.Float64(),
			Service: randomServices()[0],
		})
	}
	sum := 0.0
	for _, sample := range samples {
		sum += sample.Cost
	}
	c.monthlySummaryTenantCache = &TenantCostMonthlySummary{
		Series: samples,
		Sum:    sum,
	}
	return c.monthlySummaryTenantCache, nil
}

func (c *FakeClient) MonthlyForService(_ context.Context, teamSlug slug.Slug, environmentName, workloadName, service string) (float32, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	cc := teamSlug.String() + environmentName + workloadName + service
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
		"Valkey",
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

func serviceCostSeries(d time.Time) *ServiceCostSeries {
	if d.Before(startOfCost) {
		return &ServiceCostSeries{
			Date: scalar.Date(d),
		}
	}
	services := make([]*ServiceCostSample, 0)
	serviceNames := randomServices()
	for _, service := range serviceNames {
		services = append(services, &ServiceCostSample{
			Service: service,
			Cost:    rand.Float64(),
		})
	}
	return &ServiceCostSeries{
		Date:     scalar.Date(d),
		Services: services,
	}
}

func workloadCostSeries(ctx context.Context, d time.Time, teamSlug slug.Slug, environmentName string) *WorkloadCostSeries {
	if d.Before(startOfCost) {
		return &WorkloadCostSeries{
			Date: scalar.Date(d),
		}
	}

	workloadNames := make([]string, 0)
	for _, a := range application.ListAllForTeamInEnvironment(ctx, teamSlug, environmentName) {
		workloadNames = append(workloadNames, a.Name)
	}
	for _, j := range job.ListAllForTeamInEnvironment(ctx, teamSlug, environmentName) {
		workloadNames = append(workloadNames, j.Name)
	}
	workloads := make([]*WorkloadCostSample, len(workloadNames))
	for i, name := range workloadNames {
		workloads[i] = &WorkloadCostSample{
			TeamSlug:        teamSlug,
			EnvironmentName: environmentName,
			WorkloadName:    name,
			Cost:            rand.Float64(),
		}
	}
	return &WorkloadCostSeries{
		Date:      scalar.Date(d),
		Workloads: workloads,
	}
}

func filterDailyCost(source *TeamCostPeriod, filter *TeamCostDailyFilter) *TeamCostPeriod {
	ret := &TeamCostPeriod{}
	for _, series := range source.Series {
		newSeries := &ServiceCostSeries{
			Date: series.Date,
		}
		for _, point := range series.Services {
			if filter == nil || len(filter.Services) == 0 || slices.Contains(filter.Services, point.Service) {
				newSeries.Services = append(newSeries.Services, point)
			}
		}
		if len(newSeries.Services) > 0 {
			ret.Series = append(ret.Series, newSeries)
		}
	}

	return ret
}

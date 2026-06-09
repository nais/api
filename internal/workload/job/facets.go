package job

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
)

// Filtered returns the filtered jobs, computing it exactly once per request.
func (f *JobFacets) Filtered(ctx context.Context) []*Job {
	f.filteredOnce.Do(func() {
		f.filteredJobs = SortFilter.Filter(ctx, f.AllJobs, f.Filter)
	})
	return f.filteredJobs
}

// Environments computes environments facets for a job query.
func (f *JobFacets) Environments(ctx context.Context) ([]*model.StringFacetItem, error) {
	filtered := f.Filtered(ctx)
	items := model.ComputeEnvironmentsFacet(f.AllJobs, filtered, func(j *Job) string {
		return j.EnvironmentName
	})

	ret := make([]*model.StringFacetItem, len(items))
	for i := range items {
		ret[i] = &items[i]
	}
	return ret, nil
}

// States computes states facets for a job query.
func (f *JobFacets) States(ctx context.Context) ([]*JobStateFacetItem, error) {
	stateCounts := map[JobState]int{}
	for _, j := range f.AllJobs {
		state, err := GetState(ctx, j)
		if err != nil {
			state = JobStateUnknown
		}
		if _, ok := stateCounts[state]; !ok {
			stateCounts[state] = 0
		}
	}

	filtered := f.Filtered(ctx)
	for _, j := range filtered {
		state, err := GetState(ctx, j)
		if err != nil {
			state = JobStateUnknown
		}
		stateCounts[state]++
	}

	states := make([]JobStateFacetItem, 0, len(stateCounts))
	for state, count := range stateCounts {
		states = append(states, JobStateFacetItem{
			State: state,
			Count: count,
		})
	}
	slices.SortFunc(states, func(a, b JobStateFacetItem) int {
		return strings.Compare(a.State.String(), b.State.String())
	})

	ret := make([]*JobStateFacetItem, len(states))
	for i := range states {
		ret[i] = &states[i]
	}
	return ret, nil
}

// Labels computes labels facets for a job query.
func (f *JobFacets) Labels(ctx context.Context) ([]*model.LabelFacetItem, error) {
	filtered := f.Filtered(ctx)
	items := model.ComputeLabelsFacet(f.AllJobs, filtered, func(j *Job) []*model.ResourceLabel {
		return j.Labels
	})

	ret := make([]*model.LabelFacetItem, len(items))
	for i := range items {
		ret[i] = &items[i]
	}
	return ret, nil
}

func matchesFilter(ctx context.Context, j *Job, filter *TeamJobsFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Name != "" {
		if !strings.Contains(strings.ToLower(j.Name), strings.ToLower(filter.Name)) {
			return false
		}
	}

	if len(filter.Environments) > 0 {
		if !slices.Contains(filter.Environments, j.EnvironmentName) {
			return false
		}
	}

	if len(filter.States) > 0 {
		state, err := GetState(ctx, j)
		if err != nil {
			if !slices.Contains(filter.States, JobStateUnknown) {
				return false
			}
		} else if !slices.Contains(filter.States, state) {
			return false
		}
	}

	if !model.MatchesLabelFilters(j.Labels, filter.Labels) {
		return false
	}

	return true
}

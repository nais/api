package secret

import (
	"context"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

// Filtered returns the filtered secrets, computing it exactly once per request.
func (f *SecretFacets) Filtered(ctx context.Context) []*Secret {
	f.filteredOnce.Do(func() {
		f.filteredSecrets = SortFilter.Filter(ctx, f.AllSecrets, f.Filter)
	})
	return f.filteredSecrets
}

// Environments computes environments facets for a secret query.
func (f *SecretFacets) Environments(ctx context.Context) []model.StringFacetItem {
	filtered := f.Filtered(ctx)
	return model.ComputeEnvironmentsFacet(f.AllSecrets, filtered, func(s *Secret) string {
		return s.EnvironmentName
	})
}

// InUse computes in-use facets for a secret query.
func (f *SecretFacets) InUse(ctx context.Context) []model.BooleanFacetItem {
	inUseCounts := map[bool]int{}
	inUseSet := buildSecretInUseSet(ctx, f.AllSecrets)

	for _, s := range f.AllSecrets {
		inUseCounts[inUseSet[s.EnvironmentName+"/"+s.Name]] = 0
	}

	filtered := f.Filtered(ctx)
	for _, s := range filtered {
		inUseCounts[inUseSet[s.EnvironmentName+"/"+s.Name]]++
	}

	inUse := make([]model.BooleanFacetItem, 0, len(inUseCounts))
	for val, count := range inUseCounts {
		inUse = append(inUse, model.BooleanFacetItem{
			Value: val,
			Count: count,
		})
	}
	model.SortBooleanFacetItems(inUse)
	return inUse
}

// Labels computes labels facets for a secret query.
func (f *SecretFacets) Labels(ctx context.Context) []model.LabelFacetItem {
	filtered := f.Filtered(ctx)
	return model.ComputeLabelsFacet(f.AllSecrets, filtered, func(s *Secret) []*model.ResourceLabel {
		return s.Labels
	})
}

func buildSecretInUseSet(ctx context.Context, secrets []*Secret) map[string]bool {
	if len(secrets) == 0 {
		return nil
	}

	teamSlug := secrets[0].TeamSlug
	envs := make(map[string]bool)
	for _, s := range secrets {
		envs[s.EnvironmentName] = true
	}

	referenced := make(map[string]bool)
	for env := range envs {
		for _, app := range application.ListAllForTeamInEnvironment(ctx, teamSlug, env) {
			for _, name := range app.GetSecrets() {
				referenced[env+"/"+name] = true
			}
		}
		for _, j := range job.ListAllForTeamInEnvironment(ctx, teamSlug, env) {
			for _, name := range j.GetSecrets() {
				referenced[env+"/"+name] = true
			}
		}
	}

	return referenced
}

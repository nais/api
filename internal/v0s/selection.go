package v0s

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
)

func normalizedWorkloadMetadata(environments []string, labels []*model.LabelFilter) ([]string, []*model.LabelFilter) {
	environments = append([]string{}, environments...)
	slices.Sort(environments)
	environments = slices.Compact(environments)
	selectedLabels := make([]*model.LabelFilter, 0, len(labels))
	for _, label := range labels {
		if label != nil && label.Key != "" {
			selectedLabels = append(selectedLabels, label)
		}
	}
	slices.SortFunc(selectedLabels, compareLabelFilters)
	selectedLabels = slices.CompactFunc(selectedLabels, func(a, b *model.LabelFilter) bool { return compareLabelFilters(a, b) == 0 })
	return environments, selectedLabels
}

func workloadSelectionBinding(ctx context.Context, selected []workload.Workload, filter *VulnerabilityWorkloadFilter) (string, error) {
	type identity struct {
		Environment, Team, Name, Kind, Image string
	}
	selection := struct {
		Teams        []slug.Slug
		Environments []string
		Labels       []*model.LabelFilter
		Workloads    []identity
	}{}
	var environments []string
	var labels []*model.LabelFilter
	if filter != nil {
		selection.Teams = slices.Clone(filter.TeamScope.Resolve(ctx))
		environments, labels = filter.Environments, filter.Labels
	}
	slices.Sort(selection.Teams)
	selection.Environments, selection.Labels = normalizedWorkloadMetadata(environments, labels)
	for _, w := range selected {
		selection.Workloads = append(selection.Workloads, identity{
			Environment: w.GetEnvironmentName(), Team: w.GetTeamSlug().String(), Name: w.GetName(),
			Kind: w.GetType().String(), Image: w.GetImageString(),
		})
	}
	slices.SortFunc(selection.Workloads, func(a, b identity) int {
		return cmp.Or(cmp.Compare(a.Environment, b.Environment), cmp.Compare(a.Team, b.Team),
			cmp.Compare(a.Name, b.Name), cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Image, b.Image))
	})
	selection.Workloads = slices.Compact(selection.Workloads)
	data, err := json.Marshal(selection)
	if err != nil {
		return "", fmt.Errorf("encode vulnerability workload selection: %w", err)
	}
	return selectionKey("workload-selection", string(data)), nil
}

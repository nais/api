package checker

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/v0s"
	protoapi "github.com/nais/v0s/api/v0s/v1"
	"google.golang.org/protobuf/proto"
)

type sbomKey struct {
	env, team, name string
	kind            issue.ResourceType
}

type sbomInventory map[sbomKey]string

type sbomResult map[sbomKey]bool

type sbomCheck struct {
	workloads Workload
	v0s       v0s.WorkloadClient
}

func (s sbomCheck) inventory() (sbomInventory, error) {
	for _, env := range s.workloads.AppWatcher.EnvironmentNames() {
		if !s.workloads.AppWatcher.HasSynced(env) {
			return nil, fmt.Errorf("application inventory is not synchronized for %s", env)
		}
	}
	for _, env := range s.workloads.JobWatcher.EnvironmentNames() {
		if !s.workloads.JobWatcher.HasSynced(env) {
			return nil, fmt.Errorf("job inventory is not synchronized for %s", env)
		}
	}
	result := make(sbomInventory)
	add := func(w NaisWorkload, cluster string, kind issue.ResourceType) {
		image := w.GetEffectiveImage()
		if image == "" {
			image = w.GetImage()
		}
		result[sbomKey{environmentmapper.EnvironmentName(cluster), w.GetNamespace(), w.GetName(), kind}] = image
	}
	for _, app := range s.workloads.AppWatcher.All() {
		add(app.Obj, app.Cluster, issue.ResourceTypeApplication)
	}
	for _, job := range s.workloads.JobWatcher.All() {
		add(job.Obj, job.Cluster, issue.ResourceTypeJob)
	}
	return result, nil
}

func (s sbomCheck) collect(ctx context.Context, inventory sbomInventory) (sbomResult, error) {
	if len(inventory) == 0 {
		return sbomResult{}, nil
	}
	return v0sSBOM(ctx, s.v0s, inventory)
}

func v0sSBOM(ctx context.Context, client v0s.WorkloadClient, inventory sbomInventory) (sbomResult, error) {
	type identity struct{ env, team, name string }
	keys := make(map[identity][]sbomKey)
	for key := range inventory {
		id := identity{key.env, key.team, key.name}
		keys[id] = append(keys[id], key)
	}
	refs := make([]*protoapi.WorkloadRef, 0, len(keys))
	for id := range keys {
		refs = append(refs, &protoapi.WorkloadRef{Environment: id.env, Team: id.team, Name: id.name})
	}
	slices.SortFunc(refs, func(a, b *protoapi.WorkloadRef) int {
		return slices.Compare([]string{a.Environment, a.Team, a.Name}, []string{b.Environment, b.Team, b.Name})
	})
	result := make(sbomResult)
	for batch := range slices.Chunk(refs, 250) {
		selected := make(map[identity]bool, len(batch))
		for _, ref := range batch {
			selected[identity{ref.Environment, ref.Team, ref.Name}] = true
		}
		rows, err := v0s.ListWorkloads(ctx, client, &protoapi.ListWorkloadsRequest{
			WorkloadSelector: &protoapi.WorkloadSelector{Workloads: batch}, PageSize: 1000,
		})
		if err != nil {
			return nil, err
		}
		seen := make(map[identity]*protoapi.Workload)
		for _, row := range rows {
			id := identity{row.GetEnvironment(), row.GetTeam(), row.GetName()}
			if !selected[id] {
				return nil, fmt.Errorf("v0s returned a workload outside the SBOM selection")
			}
			if previous := seen[id]; previous != nil && !proto.Equal(previous, row) {
				return nil, fmt.Errorf("v0s returned conflicting SBOM records for %s/%s/%s", id.env, id.team, id.name)
			}
			seen[id] = row
			for _, key := range keys[id] {
				if inventory[key] == "" || inventory[key] != row.GetImageReference() || row.GetImage() == nil || row.GetResolveError() != "" {
					continue
				}
				switch row.GetImage().GetStatus() {
				case protoapi.ImageStatus_IMAGE_STATUS_NO_SBOM:
					result[key] = true
				case protoapi.ImageStatus_IMAGE_STATUS_SCANNED:
					result[key] = false
				}
			}
		}
	}
	return result, nil
}

func (s sbomCheck) issues(ctx context.Context) ([]Issue, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	inventory, err := s.inventory()
	if err != nil {
		return nil, err
	}
	result, err := s.collect(ctx, inventory)
	if err != nil {
		return nil, err
	}
	current, err := s.inventory()
	if err != nil {
		return nil, err
	}
	if !maps.Equal(inventory, current) {
		return nil, fmt.Errorf("workload inventory changed during SBOM collection")
	}
	if unknown := len(inventory) - len(result); unknown > 0 {
		s.workloads.log.WithField("unresolved_workloads", unknown).Warn("SBOM coverage unresolved")
	}
	var rows []Issue
	for key, missing := range result {
		if !missing {
			continue
		}
		rows = append(rows, Issue{
			IssueType: issue.IssueTypeMissingSBOM, ResourceName: key.name, ResourceType: key.kind,
			Team: key.team, Env: key.env, Severity: issue.SeverityWarning,
			Message: fmt.Sprintf("Image '%s' is missing a Software Bill of Materials (SBOM)", inventory[key]),
		})
	}
	return rows, nil
}

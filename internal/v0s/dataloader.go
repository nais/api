package v0s

import (
	"context"
	"fmt"
	"sync"

	"github.com/nais/api/internal/graph/loader"
	inventoryapi "github.com/nais/api/pkg/apiclient/protoapi"
	protoapi "github.com/nais/v0s/api/v0s/v1"
	"github.com/vikstrous/dataloadgen"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type contextKey int

const (
	managerKey contextKey = iota
	loadersKey
)

type workloadKey struct {
	environment, team, name string
}

type scope struct {
	team, environment, workload string
}

func (s scope) matches(k workloadKey) bool {
	return (s.team == "" || s.team == k.team) &&
		(s.environment == "" || s.environment == k.environment) &&
		(s.workload == "" || s.workload == k.name)
}

type loaders struct {
	manager           *Manager
	inventory         *dataloadgen.Loader[struct{}, map[workloadKey]string]
	workloads         *dataloadgen.Loader[scope, map[workloadKey]*protoapi.Workload]
	selectedWorkloads *dataloadgen.Loader[workloadKey, *protoapi.Workload]
	counts            *dataloadgen.Loader[string, map[string]int64]
	advisories        *dataloadgen.Loader[string, *Vulnerability]
	suppressionCounts *dataloadgen.Loader[string, int]
	workloadOverviews *dataloadgen.Loader[workloadOverviewKey, *VulnerabilityWorkloadOverview]
}

// NewLoaderContext caches inventory, coverage and grouped counts for one request.
func NewLoaderContext(ctx context.Context, inventory func(context.Context) (*inventoryapi.ListWorkloadImagesResponse, error)) context.Context {
	m, _ := ctx.Value(managerKey).(*Manager)
	l := &loaders{manager: m}
	l.workloadOverviews = dataloadgen.NewLoader(l.loadWorkloadOverviews, loader.DefaultDataLoaderOptions...)
	l.inventory = dataloadgen.NewLoader(func(ctx context.Context, keys []struct{}) ([]map[workloadKey]string, []error) {
		resp, err := inventory(ctx)
		result := make(map[workloadKey]string)
		if err == nil {
			for _, env := range resp.GetEnvironments() {
				for _, w := range env.GetWorkloads() {
					result[workloadKey{env.GetEnvironment(), w.GetTeamSlug(), w.GetName()}] = w.GetImage()
				}
			}
		}
		values := make([]map[workloadKey]string, len(keys))
		errs := make([]error, len(keys))
		for i := range keys {
			values[i], errs[i] = result, err
		}
		return values, errs
	}, loader.DefaultDataLoaderOptions...)
	l.workloads = dataloadgen.NewLoader(func(ctx context.Context, keys []scope) ([]map[workloadKey]*protoapi.Workload, []error) {
		values := make([]map[workloadKey]*protoapi.Workload, len(keys))
		errs := make([]error, len(keys))
		for i, key := range keys {
			values[i], errs[i] = l.listWorkloads(ctx, &protoapi.ListWorkloadsRequest{
				Team: key.team, Environment: key.environment, PageSize: 1000,
			})
		}
		return values, errs
	}, loader.DefaultDataLoaderOptions...)
	l.selectedWorkloads = dataloadgen.NewLoader(func(ctx context.Context, keys []workloadKey) ([]*protoapi.Workload, []error) {
		selected := make(map[workloadKey]*protoapi.Workload, len(keys))
		for _, key := range keys {
			selected[key] = nil
		}
		rows, err := l.listWorkloads(ctx, &protoapi.ListWorkloadsRequest{
			WorkloadSelector: workloadSelector(selected), PageSize: 1000,
		})
		values := make([]*protoapi.Workload, len(keys))
		errs := make([]error, len(keys))
		for i, key := range keys {
			values[i], errs[i] = rows[key], err
		}
		return values, errs
	}, loader.DefaultDataLoaderOptions...)
	l.counts = dataloadgen.NewLoader(func(ctx context.Context, keys []string) ([]map[string]int64, []error) {
		values := make([]map[string]int64, len(keys))
		errs := make([]error, len(keys))
		for i, key := range keys {
			filter := &protoapi.FindingFilter{}
			if err := proto.Unmarshal([]byte(key), filter); err != nil {
				errs[i] = fmt.Errorf("decode v0s count filter: %w", err)
				continue
			}
			resp, err := m.Vulnerabilities.CountFindings(ctx, &protoapi.CountFindingsRequest{
				Filter: filter, GroupBy: protoapi.CountFindingsRequest_GROUP_BY_WORKLOAD,
			})
			if err != nil {
				errs[i] = backendError("count v0s findings", err)
				continue
			}
			values[i] = make(map[string]int64)
			for _, bucket := range resp.GetBuckets() {
				values[i][bucket.GetKey()] += bucket.GetCount()
			}
		}
		return values, errs
	}, loader.DefaultDataLoaderOptions...)
	l.advisories = dataloadgen.NewLoader(func(ctx context.Context, keys []string) ([]*Vulnerability, []error) {
		values := make([]*Vulnerability, len(keys))
		errs := make([]error, len(keys))
		var group sync.WaitGroup
		slots := make(chan struct{}, 10)
		for i, key := range keys {
			slots <- struct{}{}
			group.Go(func() {
				defer func() { <-slots }()
				resp, err := m.Vulnerabilities.GetVulnerability(ctx, &protoapi.GetVulnerabilityRequest{Id: key})
				switch {
				case status.Code(err) == codes.NotFound:
				case err != nil:
					errs[i] = backendError("get v0s vulnerability", err)
				default:
					values[i], errs[i] = advisory(resp.GetVulnerability())
				}
			})
		}
		group.Wait()
		return values, errs
	}, loader.DefaultDataLoaderOptions...)
	l.suppressionCounts = dataloadgen.NewLoader(func(ctx context.Context, keys []string) ([]int, []error) {
		values := make([]int, len(keys))
		errs := make([]error, len(keys))
		for i, key := range keys {
			req := &protoapi.ListSuppressionsRequest{}
			if err := proto.Unmarshal([]byte(key), req); err != nil {
				errs[i] = fmt.Errorf("decode v0s suppression filter: %w", err)
				continue
			}
			inventory, err := l.inventory.Load(ctx, struct{}{})
			if err != nil {
				errs[i] = err
				continue
			}
			req.PageSize = 1000
			for {
				resp, err := m.Suppressions.ListSuppressions(ctx, req)
				if err != nil {
					errs[i] = backendError("count v0s suppressions", err)
					break
				}
				for _, item := range resp.GetSuppressions() {
					if suppressionInInventory(inventory, item) {
						values[i]++
					}
				}
				if resp.GetNextPageToken() == "" {
					break
				}
				if resp.GetNextPageToken() == req.PageToken {
					errs[i] = fmt.Errorf("v0s returned a non-advancing suppression cursor")
					break
				}
				req.PageToken = resp.GetNextPageToken()
			}
		}
		return values, errs
	}, loader.DefaultDataLoaderOptions...)
	return context.WithValue(ctx, loadersKey, l)
}

func suppressionInInventory(inventory map[workloadKey]string, item *protoapi.Suppression) bool {
	_, exists := inventory[workloadKey{item.GetEnvironment(), item.GetTeam(), item.GetWorkload()}]
	return exists
}

func fromContext(ctx context.Context) (*loaders, error) {
	l, _ := ctx.Value(loadersKey).(*loaders)
	if l == nil {
		return nil, (&Manager{}).available()
	}
	if err := l.manager.available(); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *loaders) listWorkloads(ctx context.Context, req *protoapi.ListWorkloadsRequest) (map[workloadKey]*protoapi.Workload, error) {
	result := make(map[workloadKey]*protoapi.Workload)
	for {
		resp, err := l.manager.Vulnerabilities.ListWorkloads(ctx, req)
		if err != nil {
			return nil, backendError("list v0s workloads", err)
		}
		for _, w := range resp.GetWorkloads() {
			result[workloadKey{w.GetEnvironment(), w.GetTeam(), w.GetName()}] = w
		}
		if resp.GetNextPageToken() == "" {
			return result, nil
		}
		if resp.GetNextPageToken() == req.PageToken {
			return nil, fmt.Errorf("v0s returned a non-advancing workload cursor")
		}
		req.PageToken = resp.GetNextPageToken()
	}
}

// currentWorkloads excludes removed workloads and findings for an obsolete deployed image.
func (l *loaders) currentWorkloads(ctx context.Context, s scope) (map[workloadKey]*protoapi.Workload, error) {
	inventory, err := l.inventory.Load(ctx, struct{}{})
	if err != nil {
		return nil, err
	}
	all, err := l.workloads.Load(ctx, scope{team: s.team, environment: s.environment})
	if err != nil {
		return nil, err
	}
	current := make(map[workloadKey]*protoapi.Workload)
	for key, w := range all {
		image, exists := inventory[key]
		if exists && s.matches(key) && image == w.GetImageReference() {
			current[key] = w
		}
	}
	return current, nil
}

func (l *loaders) countCurrent(ctx context.Context, filter *protoapi.FindingFilter, current map[workloadKey]*protoapi.Workload) (int64, error) {
	data, err := proto.Marshal(filter)
	if err != nil {
		return 0, fmt.Errorf("encode v0s count filter: %w", err)
	}
	buckets, err := l.counts.Load(ctx, string(data))
	if err != nil {
		return 0, err
	}
	var count int64
	for key, w := range current {
		if w.GetImage() != nil {
			count += buckets[key.environment+"/"+key.team+"/"+key.name]
		}
	}
	return count, nil
}

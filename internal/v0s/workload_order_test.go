package v0s

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	inventoryapi "github.com/nais/api/pkg/apiclient/protoapi"
	protoapi "github.com/nais/v0s/api/v0s/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type orderedWorkload struct {
	workload.Workload
	name, environment string
	kind              workload.Type
}

func (w orderedWorkload) GetName() string            { return w.name }
func (w orderedWorkload) GetEnvironmentName() string { return w.environment }
func (w orderedWorkload) GetTeamSlug() slug.Slug     { return slug.Slug("team") }
func (w orderedWorkload) GetType() workload.Type     { return w.kind }

type workloadReadActor struct{ authz.AuthenticatedUser }

func TestUniqueWorkloadOrderingUsesFilteredCountsAndIncompleteFirst(t *testing.T) {
	fields := []workload.WorkloadOrderField{
		workload.WorkloadOrderFieldVulnerabilityUniqueCritical, workload.WorkloadOrderFieldVulnerabilityUniqueHigh,
		workload.WorkloadOrderFieldVulnerabilityUniqueMedium, workload.WorkloadOrderFieldVulnerabilityUniqueLow,
		workload.WorkloadOrderFieldVulnerabilityUniqueUnknown,
	}
	for _, field := range fields {
		for _, direction := range []model.OrderDirection{model.OrderDirectionAsc, model.OrderDirectionDesc} {
			t.Run(fmt.Sprintf("%s/%s", field, direction), func(t *testing.T) {
				items := []workload.Workload{
					orderedWorkload{name: "z-high", environment: "dev"},
					orderedWorkload{name: "same", environment: "prod", kind: workload.TypeApplication},
					orderedWorkload{name: "same", environment: "dev", kind: workload.TypeJob},
					orderedWorkload{name: "same", environment: "dev", kind: workload.TypeApplication},
					orderedWorkload{name: "clean", environment: "dev"},
					orderedWorkload{name: "stale", environment: "dev"},
					orderedWorkload{name: "missing", environment: "dev"},
					orderedWorkload{name: "unresolved", environment: "dev"},
					orderedWorkload{name: "no-sbom", environment: "dev"},
					orderedWorkload{name: "failed", environment: "dev"},
				}
				calls := 0
				c := &overviewClient{list: func(req *protoapi.ListWorkloadsRequest) (*protoapi.ListWorkloadsResponse, error) {
					calls++
					if req.Overview.GetFindingFilter().GetPackageType() != "npm" || !req.Overview.GetFindingFilter().GetHasFix() {
						t.Errorf("sorting lost finding filter: %v", req.Overview)
					}
					rows := []*protoapi.Workload{}
					for _, ref := range req.WorkloadSelector.GetWorkloads() {
						if ref.Name == "missing" {
							continue
						}
						counts := &protoapi.VulnerabilitySeverityCounts{Critical: 2, High: 3, Medium: 4, Low: 5, Unknown: 6}
						if ref.Name == "z-high" {
							counts = &protoapi.VulnerabilitySeverityCounts{Critical: 7, High: 1, Medium: 9, Low: 10, Unknown: 11}
						} else if ref.Name == "clean" {
							counts = &protoapi.VulnerabilitySeverityCounts{}
						}
						row := &protoapi.Workload{
							Environment: ref.Environment, Team: ref.Team, Name: ref.Name, ImageReference: "repo/" + ref.Name + ":2",
							Image: &protoapi.Image{Status: protoapi.ImageStatus_IMAGE_STATUS_SCANNED},
							FindingCounts: &protoapi.WorkloadFindingCounts{
								FindingCount: 100, UniqueVulnerabilityCount: 50, SeverityCounts: counts,
							},
						}
						switch ref.Name {
						case "stale":
							row.ImageReference = "repo/stale:1"
						case "unresolved":
							row.Image, row.ResolveError = nil, "cannot resolve"
						case "no-sbom":
							row.Image.Status = protoapi.ImageStatus_IMAGE_STATUS_NO_SBOM
						case "failed":
							row.Image.Status = protoapi.ImageStatus_IMAGE_STATUS_FAILED
						}
						rows = append(rows, row)
					}
					return &protoapi.ListWorkloadsResponse{Workloads: rows, TotalCount: new(int64(len(rows)))}, nil
				}}
				ctx := authz.ContextWithActor(t.Context(), workloadReadActor{}, nil)
				ctx = WithManager(ctx, &Manager{Vulnerabilities: c})
				ctx = NewLoaderContext(ctx, func(context.Context) (*inventoryapi.ListWorkloadImagesResponse, error) {
					envs := map[string]*inventoryapi.EnvironmentWorkloadImages{}
					for _, w := range items {
						env := envs[w.GetEnvironmentName()]
						if env == nil {
							env = &inventoryapi.EnvironmentWorkloadImages{Environment: w.GetEnvironmentName()}
							envs[env.Environment] = env
						}
						env.Workloads = append(env.Workloads, &inventoryapi.WorkloadImage{TeamSlug: "team", Name: w.GetName(), Image: "repo/" + w.GetName() + ":2"})
					}
					result := &inventoryapi.ListWorkloadImagesResponse{}
					for _, env := range envs {
						result.Environments = append(result.Environments, env)
					}
					return result, nil
				})
				filter := &VulnerabilityFindingFilter{PackageType: new("NPM"), OnlyWithFix: new(true)}
				statuses, err := SortWorkloads(ctx, items, filter, &workload.WorkloadOrder{Field: field, Direction: direction})
				if err != nil {
					t.Fatal(err)
				}
				want := []string{"failed", "missing", "no-sbom", "stale", "unresolved", "clean", "same/dev/Application", "same/dev/Naisjob", "same/prod/Application", "z-high"}
				if direction == model.OrderDirectionDesc {
					want = append(want[:5], "z-high", "same/dev/Application", "same/dev/Naisjob", "same/prod/Application", "clean")
				}
				if field == workload.WorkloadOrderFieldVulnerabilityUniqueHigh {
					if direction == model.OrderDirectionAsc {
						want = append(want[:5], "clean", "z-high", "same/dev/Application", "same/dev/Naisjob", "same/prod/Application")
					} else {
						want = append(want[:5], "same/dev/Application", "same/dev/Naisjob", "same/prod/Application", "z-high", "clean")
					}
				}
				got := make([]string, len(items))
				for i, w := range items {
					got[i] = w.GetName()
					if w.GetName() == "same" {
						got[i] += "/" + w.GetEnvironmentName() + "/" + w.GetType().String()
					}
					overview, err := WorkloadOverview(ctx, WorkloadScope(w), &VulnerabilityFindingFilter{PackageType: new("npm"), OnlyWithFix: new(true)})
					if err != nil {
						t.Fatal(err)
					}
					if overview.Coverage.Status != statuses[i] {
						t.Errorf("coverage differs between sorting and row %s", got[i])
					}
				}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("order (-want +got):\n%s", diff)
				}
				if calls != 1 {
					t.Errorf("sorting and rows made %d backend calls, want one batch", calls)
				}
			})
		}
	}
}

func TestUniqueWorkloadSortingPropagatesBackendFailure(t *testing.T) {
	ctx := authz.ContextWithActor(t.Context(), workloadReadActor{}, nil)
	ctx = WithManager(ctx, &Manager{Vulnerabilities: &overviewClient{list: func(*protoapi.ListWorkloadsRequest) (*protoapi.ListWorkloadsResponse, error) {
		return nil, status.Error(codes.Unavailable, "KEV unavailable")
	}}})
	ctx = NewLoaderContext(ctx, func(context.Context) (*inventoryapi.ListWorkloadImagesResponse, error) {
		return &inventoryapi.ListWorkloadImagesResponse{}, nil
	})
	_, err := SortWorkloads(ctx, []workload.Workload{orderedWorkload{name: "app", environment: "dev"}}, nil,
		&workload.WorkloadOrder{Field: workload.WorkloadOrderFieldVulnerabilityUniqueCritical, Direction: model.OrderDirectionDesc})
	if err == nil {
		t.Fatal("backend failure became a successful order")
	}
}

func TestUniqueWorkloadCursorsBindNormalizedSelectionAndPreservePagination(t *testing.T) {
	scope := Scope{Team: slug.Slug("team")}
	filter := &workload.TeamWorkloadsFilter{
		Environments: []string{"prod", "dev"},
		Labels:       []*model.LabelFilter{{Key: "owner"}, {Key: "empty", Value: new("")}},
	}
	findings := &VulnerabilityFindingFilter{
		PackageType: new("NPM"), Severities: []VulnerabilitySeverity{VulnerabilitySeverityHigh, VulnerabilitySeverityCritical},
		VulnerabilityIdentifiers: []string{"CVE-2", "CVE-1"},
	}
	order := &workload.WorkloadOrder{Field: workload.WorkloadOrderFieldVulnerabilityUniqueCritical, Direction: model.OrderDirectionDesc}
	items := []workload.Workload{orderedWorkload{name: "a"}, orderedWorkload{name: "b"}, orderedWorkload{name: "c"}, orderedWorkload{name: "d"}}
	page, binding, err := ParseWorkloadPage(scope, filter, findings, order, new(2), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	first := pagination.NewConnection(pagination.Slice(items, page), page, len(items))
	if err := BindWorkloadCursors(first, binding); err != nil {
		t.Fatal(err)
	}
	equivalent := &workload.TeamWorkloadsFilter{
		Environments: []string{"dev", "prod", "dev"},
		Labels:       []*model.LabelFilter{{Key: "empty", Value: new("")}, {Key: "owner"}, {Key: "owner"}, nil, {Key: ""}},
	}
	equivalentFindings := &VulnerabilityFindingFilter{
		PackageType: new("npm"), Severities: []VulnerabilitySeverity{VulnerabilitySeverityCritical, VulnerabilitySeverityHigh, VulnerabilitySeverityHigh},
		VulnerabilityIdentifiers: []string{"CVE-1", "CVE-2", "CVE-1"},
	}
	page, _, err = ParseWorkloadPage(scope, equivalent, equivalentFindings, order, new(1), first.PageInfo.EndCursor, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	next := pagination.NewConnection(pagination.Slice(items, page), page, len(items))
	if len(next.Edges) != 1 || next.Edges[0].Node.GetName() != "c" {
		t.Fatalf("tie boundary or changed page size skipped a row: %+v", next)
	}
	for _, boundary := range []int{0, 1} {
		page, _, err = ParseWorkloadPage(scope, filter, findings, order, nil, nil, new(1), &first.Edges[boundary].Cursor)
		if err != nil {
			t.Fatal(err)
		}
		back := pagination.NewConnection(pagination.Slice(items, page), page, len(items))
		if len(back.Edges) != boundary || (boundary == 1 && back.Edges[0].Node.GetName() != "a") {
			t.Fatalf("exclusive backward boundary %d failed: %+v", boundary, back)
		}
	}
	if _, err := pagination.ParsePage(new(1), first.PageInfo.EndCursor, nil, nil); err == nil {
		t.Error("legacy pagination accepted a bound cursor")
	}
	for _, change := range []string{"team", "environment", "label", "label value", "finding filter", "severity", "direction", "legacy cursor", "version"} {
		t.Run(change, func(t *testing.T) {
			s, f, vf, o, cursor := scope, *filter, *findings, *order, *first.PageInfo.EndCursor
			f.Labels = slices.Clone(filter.Labels)
			switch change {
			case "team":
				s.Team = slug.Slug("other")
			case "environment":
				f.Environments = []string{"dev"}
			case "label":
				f.Labels = nil
			case "label value":
				f.Labels[0] = &model.LabelFilter{Key: "owner", Value: new("")}
			case "finding filter":
				vf.OnlyKnownExploited = new(true)
			case "severity":
				o.Field = workload.WorkloadOrderFieldVulnerabilityUniqueHigh
			case "direction":
				o.Direction = model.OrderDirectionAsc
			case "legacy cursor":
				cursor.Keyset = ""
			case "version":
				data, err := json.Marshal(workloadPageCursor{Version: 2, Selection: binding, Offset: cursor.Offset})
				if err != nil {
					t.Fatal(err)
				}
				cursor.Keyset = base64.RawURLEncoding.EncodeToString(data)
			}
			if _, _, err := ParseWorkloadPage(s, &f, &vf, &o, new(1), &cursor, nil, nil); err == nil {
				t.Fatal("changed selection was accepted")
			}
		})
	}
}

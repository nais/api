package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/v0s"
	v0sapi "github.com/nais/api/internal/v0s/protoapi"
	"github.com/nais/api/internal/vulnerability"
	inventoryapi "github.com/nais/api/pkg/apiclient/protoapi"
	"github.com/nais/v13s/pkg/api/vulnerabilities"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type v0sGraphActor struct{ authz.AuthenticatedUser }

type v0sGraphClient struct {
	v0sapi.VulnerabilityServiceClient
	v0sapi.SuppressionServiceClient
	err error
}

func (v0sGraphClient) ListWorkloads(context.Context, *v0sapi.ListWorkloadsRequest, ...grpc.CallOption) (*v0sapi.ListWorkloadsResponse, error) {
	return &v0sapi.ListWorkloadsResponse{Workloads: []*v0sapi.Workload{{
		Team: "team-one", Environment: "dev", Name: "app", ImageReference: "repo/app:1",
		Image: &v0sapi.Image{Name: "repo/app", Digest: "sha256:one", Status: v0sapi.ImageStatus_IMAGE_STATUS_SCANNED},
	}}}, nil
}

func (v0sGraphClient) ListFindings(context.Context, *v0sapi.ListFindingsRequest, ...grpc.CallOption) (*v0sapi.ListFindingsResponse, error) {
	return &v0sapi.ListFindingsResponse{Findings: []*v0sapi.Finding{{
		Workload:        &v0sapi.WorkloadRef{Team: "team-one", Environment: "dev", Name: "app"},
		Image:           &v0sapi.Image{Name: "repo/app", Digest: "sha256:one"},
		VulnerabilityId: "CVE-new", Purl: "pkg:npm/example@1", PackageKey: "pkg:npm/example",
		Severity: v0sapi.Severity_SEVERITY_HIGH,
	}}, Cursors: []string{"finding-1"}}, nil
}

func (v0sGraphClient) CountFindings(_ context.Context, req *v0sapi.CountFindingsRequest, _ ...grpc.CallOption) (*v0sapi.CountFindingsResponse, error) {
	f := req.GetFilter()
	if f.GetHasFix() || f.GetOnlyKnownExploited() || (len(f.GetSeverities()) > 0 && f.Severities[0] != v0sapi.Severity_SEVERITY_HIGH) {
		return &v0sapi.CountFindingsResponse{}, nil
	}
	return &v0sapi.CountFindingsResponse{Buckets: []*v0sapi.CountFindingsResponse_Bucket{{Key: "dev/team-one/app", Count: 1}}}, nil
}

func (c v0sGraphClient) GetVulnerability(context.Context, *v0sapi.GetVulnerabilityRequest, ...grpc.CallOption) (*v0sapi.GetVulnerabilityResponse, error) {
	return &v0sapi.GetVulnerabilityResponse{Vulnerability: &v0sapi.Vulnerability{
		Id: "CVE-new", Cvss: []*v0sapi.Cvss{{Source: "source", Version: "3.x"}, {Source: "source", Version: "3.1", Score: new(0.0)}},
	}}, c.err
}

func (v0sGraphClient) ListSuppressions(context.Context, *v0sapi.ListSuppressionsRequest, ...grpc.CallOption) (*v0sapi.ListSuppressionsResponse, error) {
	return &v0sapi.ListSuppressionsResponse{}, nil
}

func executeV0sQuery(t *testing.T, manager *v0s.Manager, legacy *cveAliasClient, query string) []byte {
	t.Helper()
	server := handler.New(gengql.NewExecutableSchema(gengql.Config{Resolvers: NewResolver("test", nil)}))
	server.SetErrorPresenter(apierror.GetErrorPresenter(logrus.New()))
	server.AddTransport(transport.POST{})
	ctx := authz.ContextWithActor(t.Context(), v0sGraphActor{}, nil)
	ctx = vulnerability.NewLoaderContext(ctx, &vulnerability.Manager{Client: legacy}, logrus.New())
	ctx = v0s.WithManager(ctx, manager)
	ctx = v0s.NewLoaderContext(ctx, func(context.Context) (*inventoryapi.ListWorkloadImagesResponse, error) {
		return &inventoryapi.ListWorkloadImagesResponse{Environments: []*inventoryapi.EnvironmentWorkloadImages{{
			Environment: "dev", Workloads: []*inventoryapi.WorkloadImage{{TeamSlug: "team-one", Name: "app", Image: "repo/app:1"}},
		}}}, nil
	})
	body, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/graphql", bytes.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, req)
	if recorder.Code != 200 {
		t.Fatalf("GraphQL status %d: %s", recorder.Code, recorder.Body.String())
	}
	return recorder.Body.Bytes()
}

func TestV0sQueriesCoexistWithLegacyV13s(t *testing.T) {
	client := v0sGraphClient{}
	legacy := &cveAliasClient{nodes: []*vulnerabilities.CveSummary{{Cve: &vulnerabilities.Cve{Id: "CVE-old"}}}}
	body := executeV0sQuery(t, &v0s.Manager{Vulnerabilities: client, Suppressions: client}, legacy, `query {
		vulnerabilityFindings(filter: {scope: {team: "team-one", environment: "dev", workload: "app"}}, first: 1) {
			edges { cursor node { id vulnerabilityIdentifier severity fixedVersion suppression { state } workload { team environment name } } }
			nodes { id vulnerabilityIdentifier }
			pageInfo { hasNextPage hasPreviousPage startCursor endCursor totalCount }
		}
		vulnerabilityFindingSummary { totalFindings countsBySeverity { high } fixableFindings knownExploitedFindings }
		vulnerabilityCoverage { totalWorkloads scanned pending }
		vulnerability(identifier: "CVE-new") { id identifier aliases cvss { score vector } }
		vulnerabilitySuppressions { edges { node { state } } pageInfo { hasNextPage endCursor } }
		cves(first: 1) { edges { node { __typename } } }
	}`)
	var response struct {
		Data struct {
			VulnerabilityFindings struct {
				Edges []struct {
					Cursor string
					Node   struct {
						ID                      string
						VulnerabilityIdentifier string
						Severity                string
						FixedVersion            *string
					}
				}
				Nodes    []struct{ ID, VulnerabilityIdentifier string }
				PageInfo struct {
					TotalCount                   int
					HasNextPage, HasPreviousPage bool
					StartCursor, EndCursor       string
				}
			}
			VulnerabilityFindingSummary struct{ TotalFindings, FixableFindings int }
			VulnerabilityCoverage       struct{ TotalWorkloads, Scanned int }
			Vulnerability               struct{ Cvss []struct{ Score *float64 } }
		}
		Errors []json.RawMessage
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Errors) != 0 {
		t.Fatalf("GraphQL failed: %s", body)
	}
	data := response.Data
	if len(data.VulnerabilityFindings.Edges) != 1 || data.VulnerabilityFindings.Edges[0].Node.VulnerabilityIdentifier != "CVE-new" ||
		data.VulnerabilityFindings.Edges[0].Node.Severity != "HIGH" || data.VulnerabilityFindings.Edges[0].Node.FixedVersion != nil ||
		data.VulnerabilityFindings.Edges[0].Cursor == "" || data.VulnerabilityFindingSummary.TotalFindings != 1 ||
		data.VulnerabilityFindingSummary.FixableFindings != 0 || data.VulnerabilityCoverage.TotalWorkloads != 1 ||
		data.VulnerabilityCoverage.Scanned != 1 || len(data.Vulnerability.Cvss) != 2 ||
		data.Vulnerability.Cvss[0].Score != nil || data.Vulnerability.Cvss[1].Score == nil ||
		*data.Vulnerability.Cvss[1].Score != 0 || legacy.calls.Load() != 1 {
		t.Fatalf("new and old GraphQL sources were not preserved: %s", body)
	}
	connection := data.VulnerabilityFindings
	if len(connection.Nodes) != 1 || connection.Nodes[0].ID == "" || connection.Nodes[0].ID != connection.Edges[0].Node.ID ||
		connection.Nodes[0].VulnerabilityIdentifier != connection.Edges[0].Node.VulnerabilityIdentifier ||
		connection.PageInfo.TotalCount != 1 || connection.PageInfo.HasNextPage || connection.PageInfo.HasPreviousPage ||
		connection.PageInfo.StartCursor != connection.Edges[0].Cursor || connection.PageInfo.EndCursor != connection.Edges[0].Cursor {
		t.Fatalf("connection contract was not preserved: %s", body)
	}
}

func TestV0sValidationUsesProductionErrorPresenter(t *testing.T) {
	client := v0sGraphClient{err: status.Error(codes.InvalidArgument, "alias names several vulnerabilities")}
	body := executeV0sQuery(t, &v0s.Manager{Vulnerabilities: client, Suppressions: client}, &cveAliasClient{},
		`{ vulnerability(identifier: "ambiguous-alias") { identifier } }`)
	if !strings.Contains(string(body), "alias names several vulnerabilities") || strings.Contains(string(body), "server errored out") {
		t.Fatalf("client validation became a server failure: %s", body)
	}
}

func TestUnconfiguredV0sDoesNotBreakLegacyQueries(t *testing.T) {
	legacy := &cveAliasClient{nodes: []*vulnerabilities.CveSummary{{Cve: &vulnerabilities.Cve{Id: "CVE-old"}}}}
	body := executeV0sQuery(t, nil, legacy, `{ cves(first: 1) { edges { node { __typename } } } }`)
	if strings.Contains(string(body), `"errors"`) || legacy.calls.Load() != 1 {
		t.Fatalf("legacy query depends on v0s: %s", body)
	}
	body = executeV0sQuery(t, nil, legacy, `{ vulnerabilityCoverage { totalWorkloads scanned } }`)
	if !strings.Contains(string(body), `"errors"`) || strings.Contains(string(body), `"totalWorkloads":0`) {
		t.Fatalf("unconfigured v0s returned clean coverage: %s", body)
	}
}

func TestV0sGraphQLWritesAreDisabledDuringMigration(t *testing.T) {
	for _, mutation := range []string{
		`setVulnerabilitySuppressions(input: {team: "team-one", items: [{key: {environment: "dev", workload: "app", imageName: "repo/app", packageKey: "pkg:npm/example", vulnerabilityIdentifier: "CVE-1"}, state: ACCEPTED_RISK, reason: "reason"}]}) { suppressions { state } }`,
		`deleteVulnerabilitySuppressions(input: {team: "team-one", keys: [{environment: "dev", workload: "app", imageName: "repo/app", packageKey: "pkg:npm/example", vulnerabilityIdentifier: "CVE-1"}]}) { deleted { state } }`,
	} {
		body := executeV0sQuery(t, &v0s.Manager{}, &cveAliasClient{}, "mutation { "+mutation+" }")
		if !strings.Contains(string(body), "writes are disabled during migration") {
			t.Fatalf("new writes were not migration-gated: %s", body)
		}
	}
}

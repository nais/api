package checker

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/v13s/pkg/api/vulnerabilities"
	"github.com/sirupsen/logrus"
)

type staticV13sClient struct {
	summaries []*vulnerabilities.WorkloadSummary
}

func (s staticV13sClient) ListVulnerabilitySummaries(ctx context.Context, opts ...vulnerabilities.Option) (*vulnerabilities.ListVulnerabilitySummariesResponse, error) {
	return &vulnerabilities.ListVulnerabilitySummariesResponse{Nodes: s.summaries}, nil
}

func TestVulnerabilities_ExternalIngressActNowIssue(t *testing.T) {
	tests := []struct {
		name            string
		workloadName    string
		expectedIngress string
		wantIssue       bool
	}{
		{name: "external ingress class", workloadName: "ext-app", expectedIngress: "https://ext.example.com", wantIssue: true},
		{name: "internal ingress class", workloadName: "internal-only", wantIssue: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testVulnerabilitiesExternalIngressActNowIssue(t, tt.workloadName, tt.expectedIngress, tt.wantIssue)
		})
	}
}

func testVulnerabilitiesExternalIngressActNowIssue(t *testing.T, workloadName, expectedIngress string, wantIssue bool) {
	ctx := context.Background()

	scheme, err := kubernetes.NewScheme()
	if err != nil {
		t.Fatalf("create scheme: %v", err)
	}

	ccm, err := kubernetes.CreateClusterConfigMap("nav", []string{"dev-gcp"}, nil)
	if err != nil {
		t.Fatalf("create cluster config: %v", err)
	}

	mgr, err := watcher.NewManager(scheme, ccm, logrus.New(), watcher.WithClientCreator(fake.Clients(os.DirFS("./testdata"))))
	if err != nil {
		t.Fatalf("create watcher manager: %v", err)
	}
	defer mgr.Stop()

	appWatcher := application.NewWatcher(ctx, mgr)
	ingressWatcher := application.NewIngressWatcher(ctx, mgr)

	ctxWait, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if !mgr.WaitForReady(ctxWait) {
		t.Fatal("timed out waiting for watcher manager")
	}

	workload := Workload{
		AppWatcher:     *appWatcher,
		IngressWatcher: *ingressWatcher,
		V13sClient: staticV13sClient{summaries: []*vulnerabilities.WorkloadSummary{
			{
				Workload: &vulnerabilities.Workload{Cluster: "dev-gcp", Namespace: "devteam", Type: "app", Name: workloadName},
				VulnerabilitySummary: &vulnerabilities.Summary{
					Critical:  2,
					RiskScore: 100,
					ActNow:    2,
				},
			},
			{
				Workload: &vulnerabilities.Workload{Cluster: "dev-gcp", Namespace: "devteam", Type: "app", Name: workloadName},
				VulnerabilitySummary: &vulnerabilities.Summary{
					Critical:  2,
					RiskScore: 100,
					ActNow:    2,
				},
			},
			{
				Workload: &vulnerabilities.Workload{Cluster: "dev-gcp", Namespace: "devteam", Type: "app", Name: "non-existing-workload"},
				VulnerabilitySummary: &vulnerabilities.Summary{
					Critical:  2,
					RiskScore: 100,
					ActNow:    2,
				},
			},
			{
				Workload: &vulnerabilities.Workload{Cluster: "dev-gcp", Namespace: "devteam", Type: "app", Name: workloadName},
				VulnerabilitySummary: &vulnerabilities.Summary{
					ActNow: 0,
				},
			},
		}},
		log: logrus.New(),
	}

	issues := workload.vulnerabilities(ctx)
	actNowIssues := make([]*Issue, 0)
	for i := range issues {
		if issues[i].IssueType == issue.IssueTypeExternalIngressUrgentVulnerability {
			actNowIssues = append(actNowIssues, issues[i])
		}
	}

	if !wantIssue {
		if len(actNowIssues) != 0 {
			t.Fatalf("expected 0 external ingress act-now issues, got %d", len(actNowIssues))
		}
		return
	}

	if len(actNowIssues) != 1 {
		t.Fatalf("expected 1 external ingress act-now issue, got %d", len(actNowIssues))
	}

	got := actNowIssues[0]
	if got.IssueType != issue.IssueTypeExternalIngressUrgentVulnerability {
		t.Fatalf("expected issue type %s, got %s", issue.IssueTypeExternalIngressUrgentVulnerability, got.IssueType)
	}

	if got.ResourceName != workloadName {
		t.Fatalf("expected resource %s, got %s", workloadName, got.ResourceName)
	}

	details, ok := got.IssueDetails.(issue.ExternalIngressUrgentVulnerabilityIssueDetails)
	if !ok {
		t.Fatalf("expected external ingress act-now details, got %T", got.IssueDetails)
	}

	if details.PriorityUrgent != 2 {
		t.Fatalf("expected priorityUrgent 2, got %v", details.PriorityUrgent)
	}

	if len(details.Ingresses) != 1 || details.Ingresses[0] != expectedIngress {
		t.Fatalf("expected only external ingress URL, got %+v", details.Ingresses)
	}
}

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

func TestVulnerabilities_ExternalIngressUrgentIssue(t *testing.T) {
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
			testVulnerabilitiesExternalIngressUrgentIssue(t, tt.workloadName, tt.expectedIngress, tt.wantIssue)
		})
	}
}

func testVulnerabilitiesExternalIngressUrgentIssue(t *testing.T, workloadName, expectedIngress string, wantIssue bool) {
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
					KevCount:  2,
				},
			},
			{
				Workload: &vulnerabilities.Workload{Cluster: "dev-gcp", Namespace: "devteam", Type: "app", Name: workloadName},
				VulnerabilitySummary: &vulnerabilities.Summary{
					Critical:  2,
					RiskScore: 100,
					KevCount:  2,
				},
			},
			{
				Workload: &vulnerabilities.Workload{Cluster: "dev-gcp", Namespace: "devteam", Type: "app", Name: "non-existing-workload"},
				VulnerabilitySummary: &vulnerabilities.Summary{
					Critical:  2,
					RiskScore: 100,
					KevCount:  2,
				},
			},
			{
				Workload:             &vulnerabilities.Workload{Cluster: "dev-gcp", Namespace: "devteam", Type: "app", Name: workloadName},
				VulnerabilitySummary: &vulnerabilities.Summary{},
			},
		}},
		log: logrus.New(),
	}

	issues := workload.vulnerabilities(ctx)
	urgentIssues := make([]*Issue, 0)
	for i := range issues {
		if issues[i].IssueType == issue.IssueTypeExternalIngressUrgentVulnerability {
			urgentIssues = append(urgentIssues, issues[i])
		}
	}

	if !wantIssue {
		if len(urgentIssues) != 0 {
			t.Fatalf("expected 0 external ingress urgent issues, got %d", len(urgentIssues))
		}
		if got := len(filterIssues(issues, issue.IssueTypeVulnerableImage)); got != 1 {
			t.Fatalf("expected 1 known-exploited image issue for duplicate summaries, got %d", got)
		}
		return
	}

	if len(urgentIssues) != 1 {
		t.Fatalf("expected 1 external ingress urgent issue, got %d", len(urgentIssues))
	}
	if got := len(filterIssues(issues, issue.IssueTypeVulnerableImage)); got != 0 {
		t.Fatalf("expected no known-exploited image issues when urgent exists, got %d", got)
	}

	got := urgentIssues[0]
	if got.IssueType != issue.IssueTypeExternalIngressUrgentVulnerability {
		t.Fatalf("expected issue type %s, got %s", issue.IssueTypeExternalIngressUrgentVulnerability, got.IssueType)
	}

	if got.ResourceName != workloadName {
		t.Fatalf("expected resource %s, got %s", workloadName, got.ResourceName)
	}
	if want := "Urgent: Workload '" + workloadName + "' is exposed via external ingress and has 2 known-exploited vulnerabilities"; got.Message != want {
		t.Fatalf("expected %q, got %q", want, got.Message)
	}

	details, ok := got.IssueDetails.(issue.ExternalIngressUrgentVulnerabilityIssueDetails)
	if !ok {
		t.Fatalf("expected external ingress urgent details, got %T", got.IssueDetails)
	}

	if details.PriorityUrgent != 2 {
		t.Fatalf("expected priorityUrgent 2, got %v", details.PriorityUrgent)
	}

	if len(details.Ingresses) != 1 || details.Ingresses[0] != expectedIngress {
		t.Fatalf("expected only external ingress URL, got %+v", details.Ingresses)
	}
}

func TestVulnerabilities_UrgentGatesIssueEmission(t *testing.T) {
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

	tests := []struct {
		name                   string
		workloadName           string
		kevCount               int32
		highRisk               int32
		wantVulnerableImage    bool
		wantUrgentIngress      bool
		wantExternalIngressURL string
	}{
		{
			name:                   "kev findings with confirmed exposure resolve to urgent",
			workloadName:           "ext-app",
			kevCount:               2,
			wantUrgentIngress:      true,
			wantExternalIngressURL: "https://ext.example.com",
		},
		{
			name:                "kev findings without exposure resolve to high, not urgent",
			workloadName:        "internal-only",
			kevCount:            2,
			wantVulnerableImage: true,
		},
		{
			name:                "no kev findings",
			workloadName:        "ext-app",
			kevCount:            0,
			wantVulnerableImage: false,
		},
		{
			name:                "high-risk findings without kev stay high even when exposed",
			workloadName:        "ext-app",
			kevCount:            0,
			highRisk:            3,
			wantVulnerableImage: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workload := Workload{
				AppWatcher:     *appWatcher,
				IngressWatcher: *ingressWatcher,
				V13sClient: staticV13sClient{
					summaries: []*vulnerabilities.WorkloadSummary{
						{
							Workload: &vulnerabilities.Workload{
								Cluster:   "dev-gcp",
								Namespace: "devteam",
								Type:      "app",
								Name:      tt.workloadName,
								ImageName: "test-image",
							},
							VulnerabilitySummary: &vulnerabilities.Summary{
								Critical:  2,
								RiskScore: 100,
								KevCount:  tt.kevCount,
								HighRisk:  tt.highRisk,
							},
							SbomStatus: &vulnerabilities.SbomStatusInfo{
								Status: vulnerabilities.SbomStatus_SBOM_STATUS_READY,
							},
						},
					},
				},
				log: logrus.New(),
			}

			issues := workload.vulnerabilities(ctx)

			vulnerableImageIssues := filterIssues(issues, issue.IssueTypeVulnerableImage)
			urgentIssues := filterIssues(issues, issue.IssueTypeExternalIngressUrgentVulnerability)
			assertIssueCount(t, vulnerableImageIssues, tt.wantVulnerableImage, issue.IssueTypeVulnerableImage)
			assertIssueCount(t, urgentIssues, tt.wantUrgentIngress, issue.IssueTypeExternalIngressUrgentVulnerability)

			if tt.wantUrgentIngress && urgentIssues[0].Severity != issue.SeverityCritical {
				t.Fatalf("expected urgent issue to have critical severity, got %s", urgentIssues[0].Severity)
			}
			if tt.wantUrgentIngress {
				urgentDetails, ok := urgentIssues[0].IssueDetails.(issue.ExternalIngressUrgentVulnerabilityIssueDetails)
				if !ok {
					t.Fatalf("expected urgent ingress details, got %T", urgentIssues[0].IssueDetails)
				}
				if urgentDetails.PriorityUrgent != int(tt.kevCount) {
					t.Fatalf("expected priorityUrgent %d, got %d", tt.kevCount, urgentDetails.PriorityUrgent)
				}
				if len(urgentDetails.Ingresses) != 1 || urgentDetails.Ingresses[0] != tt.wantExternalIngressURL {
					t.Fatalf("expected only %q, got %+v", tt.wantExternalIngressURL, urgentDetails.Ingresses)
				}
			}

			if !tt.wantVulnerableImage {
				return
			}

			details, ok := vulnerableImageIssues[0].IssueDetails.(issue.VulnerableImageIssueDetails)
			if !ok {
				t.Fatalf("expected vulnerable image details, got %T", vulnerableImageIssues[0].IssueDetails)
			}
			if details.Critical != 2 {
				t.Fatalf("expected critical count 2, got %d", details.Critical)
			}
			if details.RiskScore != 100 {
				t.Fatalf("expected risk score 100, got %d", details.RiskScore)
			}
			if vulnerableImageIssues[0].Severity != issue.SeverityWarning {
				t.Fatalf("expected known-exploited image issue to have warning severity, got %s", vulnerableImageIssues[0].Severity)
			}
			if want := "Image 'test-image' has 2 known-exploited vulnerabilities"; vulnerableImageIssues[0].Message != want {
				t.Fatalf("expected %q, got %q", want, vulnerableImageIssues[0].Message)
			}
		})
	}
}

func filterIssues(issues []*Issue, issueType issue.IssueType) []*Issue {
	ret := make([]*Issue, 0, len(issues))
	for _, item := range issues {
		if item.IssueType == issueType {
			ret = append(ret, item)
		}
	}
	return ret
}

func assertIssueCount(t *testing.T, issues []*Issue, want bool, issueType issue.IssueType) {
	t.Helper()

	wantCount := 0
	if want {
		wantCount = 1
	}

	if len(issues) != wantCount {
		t.Fatalf("expected %d %s issues, got %d", wantCount, issueType, len(issues))
	}
}

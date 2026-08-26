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
	criticalIssues := make([]*Issue, 0)
	for i := range issues {
		if issues[i].IssueType == issue.IssueTypeExternalIngressUrgentVulnerability {
			actNowIssues = append(actNowIssues, issues[i])
		}
		if issues[i].IssueType == issue.IssueTypeExternalIngressCriticalVulnerability {
			criticalIssues = append(criticalIssues, issues[i])
		}
	}

	if !wantIssue {
		if len(actNowIssues) != 0 {
			t.Fatalf("expected 0 external ingress act-now issues, got %d", len(actNowIssues))
		}
		if len(criticalIssues) != 0 {
			t.Fatalf("expected 0 external ingress critical issues, got %d", len(criticalIssues))
		}
		return
	}

	if len(actNowIssues) != 1 {
		t.Fatalf("expected 1 external ingress act-now issue, got %d", len(actNowIssues))
	}
	if len(criticalIssues) != 1 {
		t.Fatalf("expected 1 external ingress critical issue, got %d", len(criticalIssues))
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

	criticalDetails, ok := criticalIssues[0].IssueDetails.(issue.ExternalIngressCriticalVulnerabilityIssueDetails)
	if !ok {
		t.Fatalf("expected external ingress critical details, got %T", criticalIssues[0].IssueDetails)
	}

	if len(criticalDetails.Ingresses) != 1 || criticalDetails.Ingresses[0] != expectedIngress {
		t.Fatalf("expected only external ingress URL on critical issue, got %+v", criticalDetails.Ingresses)
	}
	if criticalDetails.CvssScore != legacyCriticalCvssScore {
		t.Fatalf("expected cvssScore %.1f on critical issue, got %v", legacyCriticalCvssScore, criticalDetails.CvssScore)
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
		actNow                 int32
		wantVulnerableImage    bool
		wantUrgentIngress      bool
		wantCriticalIngress    bool
		wantExternalIngressURL string
	}{
		{
			name:                   "external exposure with urgent findings",
			workloadName:           "ext-app",
			actNow:                 2,
			wantVulnerableImage:    true,
			wantUrgentIngress:      true,
			wantCriticalIngress:    true,
			wantExternalIngressURL: "https://ext.example.com",
		},
		{
			name:                "urgent findings without external exposure",
			workloadName:        "internal-only",
			actNow:              2,
			wantVulnerableImage: true,
		},
		{
			name:                "no urgent findings",
			workloadName:        "ext-app",
			actNow:              0,
			wantVulnerableImage: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workload := Workload{
				AppWatcher:     *appWatcher,
				IngressWatcher: *ingressWatcher,
				V13sClient: staticV13sClient{summaries: []*vulnerabilities.WorkloadSummary{
					{
						Workload: &vulnerabilities.Workload{
							Cluster:   "dev-gcp",
							Namespace: "devteam",
							Type:      "app",
							Name:      tt.workloadName,
						},
						VulnerabilitySummary: &vulnerabilities.Summary{
							Critical:  2,
							RiskScore: 100,
							ActNow:    tt.actNow,
						},
						SbomStatus: &vulnerabilities.SbomStatusInfo{
							Status: vulnerabilities.SbomStatus_SBOM_STATUS_READY,
						},
					},
				}},
				log: logrus.New(),
			}

			issues := workload.vulnerabilities(ctx)

			vulnerableImageIssues := filterIssues(issues, issue.IssueTypeVulnerableImage)
			urgentIssues := filterIssues(issues, issue.IssueTypeExternalIngressUrgentVulnerability)
			criticalIssues := filterIssues(issues, issue.IssueTypeExternalIngressCriticalVulnerability)

			assertIssueCount(t, vulnerableImageIssues, tt.wantVulnerableImage, issue.IssueTypeVulnerableImage)
			assertIssueCount(t, urgentIssues, tt.wantUrgentIngress, issue.IssueTypeExternalIngressUrgentVulnerability)
			assertIssueCount(t, criticalIssues, tt.wantCriticalIngress, issue.IssueTypeExternalIngressCriticalVulnerability)

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

			if !tt.wantUrgentIngress {
				return
			}

			urgentDetails, ok := urgentIssues[0].IssueDetails.(issue.ExternalIngressUrgentVulnerabilityIssueDetails)
			if !ok {
				t.Fatalf("expected urgent ingress details, got %T", urgentIssues[0].IssueDetails)
			}
			if urgentDetails.PriorityUrgent != int(tt.actNow) {
				t.Fatalf("expected priorityUrgent %d, got %d", tt.actNow, urgentDetails.PriorityUrgent)
			}
			if len(urgentDetails.Ingresses) != 1 || urgentDetails.Ingresses[0] != tt.wantExternalIngressURL {
				t.Fatalf("expected only %q, got %+v", tt.wantExternalIngressURL, urgentDetails.Ingresses)
			}

			criticalDetails, ok := criticalIssues[0].IssueDetails.(issue.ExternalIngressCriticalVulnerabilityIssueDetails)
			if !ok {
				t.Fatalf("expected critical ingress details, got %T", criticalIssues[0].IssueDetails)
			}
			if criticalDetails.CvssScore != legacyCriticalCvssScore {
				t.Fatalf("expected cvssScore %.1f, got %v", legacyCriticalCvssScore, criticalDetails.CvssScore)
			}
			if len(criticalDetails.Ingresses) != 1 || criticalDetails.Ingresses[0] != tt.wantExternalIngressURL {
				t.Fatalf("expected only %q on critical issue, got %+v", tt.wantExternalIngressURL, criticalDetails.Ingresses)
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

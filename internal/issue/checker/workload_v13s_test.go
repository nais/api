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
	workloads []*vulnerabilities.WorkloadForVulnerability
}

func (s staticV13sClient) ListVulnerabilitySummaries(ctx context.Context, opts ...vulnerabilities.Option) (*vulnerabilities.ListVulnerabilitySummariesResponse, error) {
	return &vulnerabilities.ListVulnerabilitySummariesResponse{}, nil
}

func (s staticV13sClient) ListWorkloadsForVulnerability(ctx context.Context, vulnerabilityFilter vulnerabilities.VulnerabilityFilter, opts ...vulnerabilities.Option) (*vulnerabilities.ListWorkloadsForVulnerabilityResponse, error) {
	return &vulnerabilities.ListWorkloadsForVulnerabilityResponse{Nodes: s.workloads}, nil
}

func TestVulnerabilities_ExternalIngressCriticalIssue(t *testing.T) {
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
		V13sClient: staticV13sClient{workloads: []*vulnerabilities.WorkloadForVulnerability{
			{
				WorkloadRef:   &vulnerabilities.Workload{Cluster: "dev-gcp", Namespace: "devteam", Type: "app", Name: "ext-app"},
				Vulnerability: &vulnerabilities.Vulnerability{CvssScore: new(10.0)},
			},
			{
				WorkloadRef:   &vulnerabilities.Workload{Cluster: "dev-gcp", Namespace: "devteam", Type: "app", Name: "ext-app"},
				Vulnerability: &vulnerabilities.Vulnerability{CvssScore: new(10.0)},
			},
			{
				WorkloadRef:   &vulnerabilities.Workload{Cluster: "dev-gcp", Namespace: "devteam", Type: "app", Name: "internal-only"},
				Vulnerability: &vulnerabilities.Vulnerability{CvssScore: new(10.0)},
			},
			{
				WorkloadRef:   &vulnerabilities.Workload{Cluster: "dev-gcp", Namespace: "devteam", Type: "app", Name: "ext-app"},
				Vulnerability: &vulnerabilities.Vulnerability{CvssScore: new(9.9)},
			},
		}},
		log: logrus.New(),
	}

	issues := workload.vulnerabilities(ctx)

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}

	got := issues[0]
	if got.IssueType != issue.IssueTypeExternalIngressCriticalVulnerability {
		t.Fatalf("expected issue type %s, got %s", issue.IssueTypeExternalIngressCriticalVulnerability, got.IssueType)
	}

	if got.ResourceName != "ext-app" {
		t.Fatalf("expected resource ext-app, got %s", got.ResourceName)
	}

	details, ok := got.IssueDetails.(issue.ExternalIngressCriticalVulnerabilityIssueDetails)
	if !ok {
		t.Fatalf("expected external ingress critical details, got %T", got.IssueDetails)
	}

	if details.CvssScore != 10.0 {
		t.Fatalf("expected CVSS 10.0, got %v", details.CvssScore)
	}

	if len(details.Ingresses) != 1 || details.Ingresses[0] != "https://ext.example.com" {
		t.Fatalf("expected only external ingress URL, got %+v", details.Ingresses)
	}
}

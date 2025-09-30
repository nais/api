package checker

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/issue"
	"github.com/nais/v13s/pkg/api/vulnerabilities"
)

type V13sClient interface {
	ListVulnerabilitySummaries(ctx context.Context, opts ...vulnerabilities.Option) (*vulnerabilities.ListVulnerabilitySummariesResponse, error)
}

type fakeV13sClient struct{}

func (f fakeV13sClient) ListVulnerabilitySummaries(ctx context.Context, opts ...vulnerabilities.Option) (*vulnerabilities.ListVulnerabilitySummariesResponse, error) {
	return &vulnerabilities.ListVulnerabilitySummariesResponse{
		Nodes: []*vulnerabilities.WorkloadSummary{
			{
				Id: "1",
				Workload: &vulnerabilities.Workload{
					Name:      "vulnerable",
					Namespace: "devteam",
					Cluster:   "dev-gcp",
					Type:      "app",
					ImageName: "vulnerable-image",
					ImageTag:  "tag1",
				},
				VulnerabilitySummary: &vulnerabilities.Summary{
					HasSbom:   true,
					Critical:  5,
					RiskScore: 250,
				},
			},
			{
				Id: "2",
				Workload: &vulnerabilities.Workload{
					Name:      "missing-sbom",
					Namespace: "devteam",
					Cluster:   "dev-gcp",
					Type:      "app",
					ImageName: "missing-sbom-image",
					ImageTag:  "tag1",
				},
				VulnerabilitySummary: &vulnerabilities.Summary{
					HasSbom: false,
				},
			},
			{
				Id: "3",
				Workload: &vulnerabilities.Workload{
					Name:      "vulnerable",
					Namespace: "myteam",
					Cluster:   "dev-gcp",
					Type:      "app",
					ImageName: "vulnerable-image",
					ImageTag:  "tag1",
				},
				VulnerabilitySummary: &vulnerabilities.Summary{
					HasSbom:   true,
					Critical:  5,
					RiskScore: 250,
				},
			},
			{
				Id: "4",
				Workload: &vulnerabilities.Workload{
					Name:      "missing-sbom",
					Namespace: "myteam",
					Cluster:   "dev-gcp",
					Type:      "app",
					ImageName: "missing-sbom-image",
					ImageTag:  "tag1",
				},
				VulnerabilitySummary: &vulnerabilities.Summary{
					HasSbom: false,
				},
			},
			{
				Id: "5",
				Workload: &vulnerabilities.Workload{
					Name:      "missing-app",
					Namespace: "myteam",
					Cluster:   "dev-gcp",
					Type:      "app",
					ImageName: "some-image",
					ImageTag:  "tag1",
				},
				VulnerabilitySummary: &vulnerabilities.Summary{
					HasSbom: false,
				},
			},
		},
	}, nil
}

func (w Workload) vulnerabilities(ctx context.Context) []*Issue {
	mapType := func(s string) (issue.ResourceType, bool) {
		if s == "job" {
			return issue.ResourceTypeJob, true
		}

		if s == "app" {
			return issue.ResourceTypeApplication, true
		}

		return "", false
	}

	resp, err := w.V13sClient.ListVulnerabilitySummaries(ctx, vulnerabilities.Limit(69000)) // unlimited
	if err != nil {
		w.log.WithError(err).Error("fetch image vulnerability summaries")
		return nil
	}

	var ret []*Issue

	for _, node := range resp.GetNodes() {
		workloadType, ok := mapType(node.Workload.GetType())
		if !ok {
			continue
		}

		if !w.exists(node, workloadType) {
			continue
		}

		if node.VulnerabilitySummary.Critical > 0 || node.VulnerabilitySummary.RiskScore > 100 {
			ret = append(ret, &Issue{
				IssueType:    issue.IssueTypeVulnerableImage,
				ResourceType: workloadType,
				ResourceName: node.Workload.GetName(),
				Team:         node.Workload.GetNamespace(),
				Env:          environmentmapper.EnvironmentName(node.Workload.GetCluster()),
				Severity:     issue.SeverityWarning,
				Message: fmt.Sprintf(
					"Image '%s' has %d critical vulnerabilities and a risk score of %d",
					node.Workload.ImageName,
					node.VulnerabilitySummary.Critical,
					node.VulnerabilitySummary.RiskScore,
				),
				IssueDetails: issue.VulnerableImageIssueDetails{
					Critical:  int(node.VulnerabilitySummary.Critical),
					RiskScore: int(node.VulnerabilitySummary.RiskScore),
				},
			})
		}

		if !node.VulnerabilitySummary.HasSbom {
			ret = append(ret, &Issue{
				IssueType:    issue.IssueTypeMissingSBOM,
				ResourceType: workloadType,
				ResourceName: node.Workload.GetName(),
				Team:         node.Workload.GetNamespace(),
				Env:          environmentmapper.EnvironmentName(node.Workload.GetCluster()),
				Severity:     issue.SeverityWarning,
				Message: fmt.Sprintf(
					"Image '%s:%s' is missing a Software Bill of Materials (SBOM)",
					node.Workload.ImageName,
					node.Workload.ImageTag,
				),
			})
		}
	}

	return ret
}

func (w Workload) exists(node *vulnerabilities.WorkloadSummary, workloadType issue.ResourceType) bool {
	env := environmentmapper.EnvironmentName(node.Workload.GetCluster())

	if workloadType == issue.ResourceTypeJob {
		job, err := w.JobWatcher.Get(env, node.Workload.GetNamespace(), node.Workload.GetName())
		if err != nil || job == nil {
			return false
		}
	}

	if workloadType == issue.ResourceTypeApplication {
		app, err := w.AppWatcher.Get(env, node.Workload.GetNamespace(), node.Workload.GetName())
		if err != nil || app == nil {
			return false
		}
	}
	return true
}

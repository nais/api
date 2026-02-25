package checker

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/kubernetes/watcher"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	"github.com/nais/v13s/pkg/api/vulnerabilities"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/utils/ptr"
)

const (
	externalIngressClassName = "nais-ingress-external"
	v13sQueryLimit           = 69000
)

type V13sClient interface {
	ListVulnerabilitySummaries(ctx context.Context, opts ...vulnerabilities.Option) (*vulnerabilities.ListVulnerabilitySummariesResponse, error)
	ListWorkloadsForVulnerability(ctx context.Context, vulnerabilityFilter vulnerabilities.VulnerabilityFilter, opts ...vulnerabilities.Option) (*vulnerabilities.ListWorkloadsForVulnerabilityResponse, error)
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

func (f fakeV13sClient) ListWorkloadsForVulnerability(ctx context.Context, vulnerabilityFilter vulnerabilities.VulnerabilityFilter, opts ...vulnerabilities.Option) (*vulnerabilities.ListWorkloadsForVulnerabilityResponse, error) {
	return &vulnerabilities.ListWorkloadsForVulnerabilityResponse{}, nil
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

	resp, err := w.V13sClient.ListVulnerabilitySummaries(ctx, vulnerabilities.Limit(v13sQueryLimit))
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

	cvss := 10.0
	workloadsForVulnerability, err := w.V13sClient.ListWorkloadsForVulnerability(
		ctx,
		vulnerabilities.VulnerabilityFilter{CvssScore: &cvss},
		vulnerabilities.Limit(v13sQueryLimit),
		vulnerabilities.ExcludeClustersFilter("management"),
	)
	if err != nil {
		w.log.WithError(err).Error("fetch workloads for vulnerabilities with cvss score")
		return ret
	}

	seen := map[string]struct{}{}
	for _, node := range workloadsForVulnerability.GetNodes() {
		workloadRef := node.GetWorkloadRef()
		vulnerability := node.GetVulnerability()
		if workloadRef == nil || vulnerability == nil {
			continue
		}

		if vulnerability.GetCvssScore() != cvss {
			continue
		}

		workloadType, ok := mapType(workloadRef.GetType())
		if !ok || workloadType != issue.ResourceTypeApplication {
			continue
		}

		env := environmentmapper.EnvironmentName(workloadRef.GetCluster())
		key := fmt.Sprintf("%s/%s/%s", env, workloadRef.GetNamespace(), workloadRef.GetName())
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		app, err := w.AppWatcher.Get(env, workloadRef.GetNamespace(), workloadRef.GetName())
		if err != nil || app == nil {
			continue
		}

		externalIngresses := w.getExternalIngresses(app, env)
		if len(externalIngresses) == 0 {
			continue
		}

		ret = append(ret, &Issue{
			IssueType:    issue.IssueTypeExternalIngressCriticalVulnerability,
			ResourceType: workloadType,
			ResourceName: workloadRef.GetName(),
			Team:         workloadRef.GetNamespace(),
			Env:          env,
			Severity:     issue.SeverityCritical,
			Message: fmt.Sprintf(
				"Workload with external ingresses %s has a vulnerability with CVSS score %.1f",
				strings.Join(externalIngresses, ", "),
				cvss,
			),
			IssueDetails: issue.ExternalIngressCriticalVulnerabilityIssueDetails{
				CvssScore: cvss,
				Ingresses: externalIngresses,
			},
		})
	}

	return ret
}

func (w Workload) getExternalIngresses(app *nais_io_v1alpha1.Application, env string) []string {
	externalIngresses := make([]string, 0, len(app.Spec.Ingresses))

	selector := labels.NewSelector()
	req, err := labels.NewRequirement("app", selection.Equals, []string{app.Name})
	if err != nil {
		w.log.WithError(err).WithField("app", app.Name).WithField("namespace", app.Namespace).WithField("env", env).Error("create label requirement for ingress lookup")
		return externalIngresses
	}
	selector = selector.Add(*req)

	ingresses := w.IngressWatcher.GetByNamespace(app.Namespace, watcher.WithLabels(selector), watcher.InCluster(env))

	for _, ingress := range app.Spec.Ingresses {
		ingressURL := string(ingress)

		if strings.TrimSpace(ingressURL) == "" {
			continue
		}

		uri, err := url.Parse(ingressURL)
		if err != nil {
			continue
		}

		if strings.TrimSpace(uri.Host) == "" {
			continue
		}

		if hasExternalIngressClass(ingresses, uri.Host) {
			externalIngresses = append(externalIngresses, ingressURL)
		}
	}

	return externalIngresses
}

func hasExternalIngressClass(ings []*watcher.EnvironmentWrapper[*netv1.Ingress], host string) bool {
	for _, ing := range ings {
		for _, rule := range ing.Obj.Spec.Rules {
			if rule.Host != host {
				continue
			}

			if ptr.Deref(ing.Obj.Spec.IngressClassName, "") == externalIngressClassName {
				return true
			}
		}
	}

	return false
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

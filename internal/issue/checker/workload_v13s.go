package checker

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/issue"
	"github.com/nais/v13s/pkg/api/vulnerabilities"
	"k8s.io/utils/ptr"
)

const (
	externalIngressClassName = "nais-ingress-external"
	v13sQueryLimit           = 69000
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
					Critical:  5,
					RiskScore: 250,
					ActNow:    2,
					HighRisk:  3,
				},
				SbomStatus: &vulnerabilities.SbomStatusInfo{
					Status: vulnerabilities.SbomStatus_SBOM_STATUS_READY,
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
				SbomStatus: &vulnerabilities.SbomStatusInfo{
					Status: vulnerabilities.SbomStatus_SBOM_STATUS_NO_SBOM,
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
					Critical:  5,
					RiskScore: 250,
					ActNow:    2,
					HighRisk:  3,
				},
				SbomStatus: &vulnerabilities.SbomStatusInfo{
					Status: vulnerabilities.SbomStatus_SBOM_STATUS_READY,
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
				SbomStatus: &vulnerabilities.SbomStatusInfo{
					Status: vulnerabilities.SbomStatus_SBOM_STATUS_NO_SBOM,
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
				SbomStatus: &vulnerabilities.SbomStatusInfo{
					Status: vulnerabilities.SbomStatus_SBOM_STATUS_NO_SBOM,
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

		summary := node.VulnerabilitySummary
		if summary != nil && summary.ActNow > 0 {
			ret = append(ret, &Issue{
				IssueType:    issue.IssueTypeVulnerableImage,
				ResourceType: workloadType,
				ResourceName: node.Workload.GetName(),
				Team:         node.Workload.GetNamespace(),
				Env:          environmentmapper.EnvironmentName(node.Workload.GetCluster()),
				Severity:     issue.SeverityCritical,
				Message: fmt.Sprintf(
					"Image '%s' has %d ACT_NOW vulnerabilities",
					node.Workload.ImageName,
					summary.ActNow,
				),
				IssueDetails: issue.VulnerableImageIssueDetails{
					Critical:  int(summary.Critical),
					RiskScore: int(summary.RiskScore),
				},
			})
		}

		sbomStatus := node.GetSbomStatus().GetStatus()
		if sbomStatus != vulnerabilities.SbomStatus_SBOM_STATUS_READY &&
			sbomStatus != vulnerabilities.SbomStatus_SBOM_STATUS_PROCESSING &&
			sbomStatus != vulnerabilities.SbomStatus_SBOM_STATUS_UNSPECIFIED {
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

	externalIngressesByWorkload := w.externalIngressesByWorkload()

	seenActNow := map[string]struct{}{}
	for _, node := range resp.GetNodes() {
		workloadType, ok := mapType(node.Workload.GetType())
		if !ok || workloadType != issue.ResourceTypeApplication {
			continue
		}

		if node.VulnerabilitySummary == nil || node.VulnerabilitySummary.ActNow == 0 {
			continue
		}

		env := environmentmapper.EnvironmentName(node.Workload.GetCluster())
		key := workloadKey(env, node.Workload.GetNamespace(), node.Workload.GetName())
		if _, exists := seenActNow[key]; exists {
			continue
		}

		externalIngresses := externalIngressesByWorkload[key]
		if len(externalIngresses) == 0 {
			continue
		}
		seenActNow[key] = struct{}{}

		ret = append(ret, &Issue{
			IssueType:    issue.IssueTypeExternalIngressActNowVulnerability,
			ResourceType: workloadType,
			ResourceName: node.Workload.GetName(),
			Team:         node.Workload.GetNamespace(),
			Env:          env,
			Severity:     issue.SeverityCritical,
			Message: fmt.Sprintf(
				"Workload with external ingresses %s has %d ACT_NOW vulnerabilities",
				strings.Join(externalIngresses, ", "),
				node.VulnerabilitySummary.ActNow,
			),
			IssueDetails: issue.ExternalIngressActNowVulnerabilityIssueDetails{
				PriorityActNow: int(node.VulnerabilitySummary.ActNow),
				Ingresses:      externalIngresses,
			},
		})
	}

	return ret
}

func (w Workload) externalIngressesByWorkload() map[string][]string {
	ret := map[string][]string{}
	externalHostsByWorkload := w.externalIngressHostsByWorkload()

	for _, app := range w.AppWatcher.All() {
		env := environmentmapper.EnvironmentName(app.Cluster)
		hosts := externalHostsByWorkload[workloadKey(env, app.Obj.GetNamespace(), app.Obj.GetName())]
		if len(hosts) == 0 {
			continue
		}

		externalIngresses := make([]string, 0, len(app.Obj.Spec.Ingresses))
		for _, ingress := range app.Obj.Spec.Ingresses {
			ingressURL := string(ingress)

			if strings.TrimSpace(ingressURL) == "" {
				continue
			}

			uri, err := url.Parse(ingressURL)
			if err != nil {
				continue
			}

			host := strings.TrimSpace(uri.Hostname())
			if host == "" {
				continue
			}

			if _, ok := hosts[host]; ok {
				externalIngresses = append(externalIngresses, ingressURL)
			}
		}

		if len(externalIngresses) == 0 {
			continue
		}

		ret[workloadKey(env, app.Obj.GetNamespace(), app.Obj.GetName())] = externalIngresses
	}

	return ret
}

func (w Workload) externalIngressHostsByWorkload() map[string]map[string]struct{} {
	ret := map[string]map[string]struct{}{}

	for _, ing := range w.IngressWatcher.All() {
		if ptr.Deref(ing.Obj.Spec.IngressClassName, "") != externalIngressClassName {
			continue
		}

		appName := strings.TrimSpace(ing.Obj.GetLabels()["app"])
		if appName == "" {
			continue
		}

		env := environmentmapper.EnvironmentName(ing.Cluster)
		key := workloadKey(env, ing.Obj.GetNamespace(), appName)
		hosts, ok := ret[key]
		if !ok {
			hosts = map[string]struct{}{}
			ret[key] = hosts
		}

		for _, rule := range ing.Obj.Spec.Rules {
			host := strings.TrimSpace(rule.Host)
			if host == "" {
				continue
			}
			hosts[host] = struct{}{}
		}
	}

	return ret
}

func workloadKey(env, namespace, name string) string {
	return fmt.Sprintf("%s/%s/%s", env, namespace, name)
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

package checker

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/vulnerability"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/v13s/pkg/api/vulnerabilities"
	"k8s.io/utils/ptr"
)

const (
	v13sQueryLimit          = 69000
	legacyCriticalCvssScore = 9.0
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
					HighRisk:  3,
					KevCount:  2,
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
					HighRisk:  3,
					KevCount:  2,
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
		env := environmentmapper.EnvironmentName(node.Workload.GetCluster())
		kevCount := int(summary.GetKevCount())
		if summary != nil && kevCount > 0 {
			ret = append(ret, &Issue{
				IssueType:    issue.IssueTypeVulnerableImage,
				ResourceType: workloadType,
				ResourceName: node.Workload.GetName(),
				Team:         node.Workload.GetNamespace(),
				Env:          env,
				Severity:     issue.SeverityCritical,
				Message: fmt.Sprintf(
					"Image '%s' has %d known-exploited vulnerabilities",
					node.Workload.ImageName,
					kevCount,
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
				Env:          env,
				Severity:     issue.SeverityWarning,
				Message: fmt.Sprintf(
					"Image '%s:%s' is missing a Software Bill of Materials (SBOM)",
					node.Workload.ImageName,
					node.Workload.ImageTag,
				),
			})
		}
	}

	ingress := w.ingressExposureByWorkload()

	seenUrgent := map[string]struct{}{}
	for _, node := range resp.GetNodes() {
		workloadRef := node.GetWorkload()
		if workloadRef == nil {
			continue
		}

		workloadType, ok := mapType(workloadRef.GetType())
		if !ok || workloadType != issue.ResourceTypeApplication {
			continue
		}

		env := environmentmapper.EnvironmentName(workloadRef.GetCluster())
		key := workloadKey(env, workloadRef.GetNamespace(), workloadRef.GetName())

		kevCount := int(node.GetVulnerabilitySummary().GetKevCount())
		if kevCount == 0 {
			continue
		}

		if _, exists := seenUrgent[key]; exists {
			continue
		}

		exposure := vulnerability.ResolveWorkloadInternetExposure(ingress.classNames[key], false)
		priority, _ := vulnerability.ResolvePriority(
			vulnerabilities.Priority_PRIORITY_HIGH,
			true,
			vulnerability.ImageVulnerabilitySeverityUnassigned,
			false,
			exposure,
		)
		if priority != vulnerability.CVEPriorityUrgent {
			continue
		}

		externalIngresses := ingress.exposedURLs[key]
		if len(externalIngresses) == 0 {
			continue
		}
		seenUrgent[key] = struct{}{}

		ret = append(ret, &Issue{
			IssueType:    issue.IssueTypeExternalIngressUrgentVulnerability,
			ResourceType: workloadType,
			ResourceName: workloadRef.GetName(),
			Team:         workloadRef.GetNamespace(),
			Env:          env,
			Severity:     issue.SeverityCritical,
			Message: fmt.Sprintf(
				"Workload '%s' (exposed via external ingress) has %d urgent vulnerabilities",
				workloadRef.GetName(),
				kevCount,
			),
			IssueDetails: issue.ExternalIngressUrgentVulnerabilityIssueDetails{
				PriorityUrgent: kevCount,
				Ingresses:      externalIngresses,
			},
		})

		ret = append(ret, &Issue{
			IssueType:    issue.IssueTypeExternalIngressCriticalVulnerability,
			ResourceType: workloadType,
			ResourceName: workloadRef.GetName(),
			Team:         workloadRef.GetNamespace(),
			Env:          env,
			Severity:     issue.SeverityCritical,
			Message: fmt.Sprintf(
				"Workload '%s' (exposed via external ingress) has %d urgent vulnerabilities",
				workloadRef.GetName(),
				kevCount,
			),
			IssueDetails: issue.ExternalIngressCriticalVulnerabilityIssueDetails{
				CvssScore: legacyCriticalCvssScore,
				Ingresses: externalIngresses,
			},
		})
	}

	return ret
}

type ingressExposure struct {
	classNames  map[string][]string
	exposedURLs map[string][]string
}

func (w Workload) ingressExposureByWorkload() ingressExposure {
	classNames := map[string][]string{}
	exposedHosts := map[string]map[string]struct{}{}

	for _, ing := range w.IngressWatcher.All() {
		appName := strings.TrimSpace(ing.Obj.GetLabels()["app"])
		if appName == "" {
			continue
		}

		env := environmentmapper.EnvironmentName(ing.Cluster)
		key := workloadKey(env, ing.Obj.GetNamespace(), appName)

		className := ptr.Deref(ing.Obj.Spec.IngressClassName, "")
		classNames[key] = append(classNames[key], className)

		ingressType := application.ClassifyIngressClassName(className)
		if ingressType != application.IngressTypeExternal && ingressType != application.IngressTypeAuthenticated {
			continue
		}

		hosts, ok := exposedHosts[key]
		if !ok {
			hosts = map[string]struct{}{}
			exposedHosts[key] = hosts
		}

		for _, rule := range ing.Obj.Spec.Rules {
			host := strings.TrimSpace(rule.Host)
			if host == "" {
				continue
			}
			hosts[host] = struct{}{}
		}
	}

	exposedIngresses := map[string][]string{}
	for _, app := range w.AppWatcher.All() {
		env := environmentmapper.EnvironmentName(app.Cluster)
		key := workloadKey(env, app.Obj.GetNamespace(), app.Obj.GetName())
		hosts := exposedHosts[key]
		if len(hosts) == 0 {
			continue
		}

		urls := make([]string, 0, len(app.Obj.Spec.Ingresses))
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
				urls = append(urls, ingressURL)
			}
		}

		if len(urls) == 0 {
			continue
		}

		exposedIngresses[key] = urls
	}

	return ingressExposure{classNames: classNames, exposedURLs: exposedIngresses}
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

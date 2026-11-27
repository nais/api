package fake

import (
	"context"
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"

	inventoryapi "github.com/nais/api/pkg/apiclient/protoapi"
	protoapi "github.com/nais/v0s/api/v0s/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (c *Client) data(ctx context.Context) ([]*protoapi.Workload, []*protoapi.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	inventory, err := c.inventory.ListImages(ctx, &inventoryapi.ListWorkloadImagesRequest{})
	if err != nil {
		return nil, nil, err
	}
	var workloads []*protoapi.Workload
	var findings []*protoapi.Finding
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, env := range inventory.GetEnvironments() {
		for _, item := range env.Workloads {
			w := &protoapi.Workload{
				Environment: env.Environment, Team: item.TeamSlug, Name: item.Name, ImageReference: item.Image,
			}
			workloads = append(workloads, w)
			if item.Image == "" {
				w.ResolveError = "No image configured (fake data)"
				continue
			}
			name, digest, hasDigest := strings.Cut(item.Image, "@")
			if colon := strings.LastIndex(name, ":"); colon > strings.LastIndex(name, "/") {
				name = name[:colon]
			}
			if !hasDigest {
				digest = fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(item.Image)))
			}
			w.Image = &protoapi.Image{
				Name: name, Digest: digest, Status: protoapi.ImageStatus_IMAGE_STATUS_SCANNED,
				ScannedAt: timestamppb.New(c.scannedAt),
			}
			if strings.Contains(item.Name, "no-sbom") {
				w.Image.Status, w.Image.ScannedAt = protoapi.ImageStatus_IMAGE_STATUS_NO_SBOM, nil
				continue
			}
			if item.Name == "no-errors" || item.Name == "priority-none" {
				continue
			}
			for _, f := range sampleFindings() {
				f.Workload = &protoapi.WorkloadRef{Environment: w.Environment, Team: w.Team, Name: w.Name}
				f.Image = clone(w.Image)
				if s := c.suppressions[findingSuppressionKey(f)]; s != nil {
					f.Suppression = clone(s)
				}
				findings = append(findings, f)
			}
		}
	}
	slices.SortFunc(workloads, func(a, b *protoapi.Workload) int {
		return strings.Compare(a.Environment+"/"+a.Team+"/"+a.Name, b.Environment+"/"+b.Team+"/"+b.Name)
	})
	return workloads, findings, nil
}

func sampleFindings() []*protoapi.Finding {
	var findings []*protoapi.Finding
	for i, sample := range []struct {
		severity   protoapi.Severity
		packageKey string
		fixed      string
		kev        bool
	}{
		{protoapi.Severity_SEVERITY_CRITICAL, "pkg:golang/example.org/demo", "1.2.0", true},
		{protoapi.Severity_SEVERITY_HIGH, "pkg:npm/example-lib", "2.0.0", false},
		{protoapi.Severity_SEVERITY_MEDIUM, "pkg:maven/org.example/demo", "", false},
		{protoapi.Severity_SEVERITY_LOW, "pkg:pypi/example-lib", "1.1.0", false},
		{protoapi.Severity_SEVERITY_UNKNOWN, "pkg:npm/another-example", "", false},
	} {
		findings = append(findings, &protoapi.Finding{
			VulnerabilityId: fmt.Sprintf("CVE-2026-%04d", 1000+i),
			PackageKey:      sample.packageKey, Purl: sample.packageKey + "@1.0.0", InstalledVersion: "1.0.0",
			FixedVersion: sample.fixed, Severity: sample.severity, KnownExploited: sample.kev,
			SeveritySource: "fake", DataSource: "fake",
			Epss: &protoapi.Epss{Score: 0.8 / float64(i+1), Date: "2026-01-01"},
		})
	}
	return findings
}

func packageType(key string) string {
	value, _, _ := strings.Cut(strings.TrimPrefix(key, "pkg:"), "/")
	return value
}

func alias(id string) string {
	return "FAKE-" + id
}

func matchesVulnerability(canonical, id string) bool {
	return id == canonical || id == alias(canonical)
}

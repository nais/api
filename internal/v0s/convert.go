package v0s

import (
	"fmt"
	"time"

	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
	protoapi "github.com/nais/v0s/api/v0s/v1"
)

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func epss(value *protoapi.Epss) (*VulnerabilityEpss, error) {
	if value == nil {
		return nil, nil
	}
	date, err := time.Parse(time.DateOnly, value.GetDate())
	if err != nil {
		return nil, fmt.Errorf("invalid v0s EPSS date: %w", err)
	}
	if value.GetScore() < 0 || value.GetScore() > 1 {
		return nil, fmt.Errorf("invalid v0s EPSS score")
	}
	return &VulnerabilityEpss{Score: value.GetScore(), Date: scalar.NewDate(date)}, nil
}

func severity(value protoapi.Severity) VulnerabilitySeverity {
	switch value {
	case protoapi.Severity_SEVERITY_LOW:
		return VulnerabilitySeverityLow
	case protoapi.Severity_SEVERITY_MEDIUM:
		return VulnerabilitySeverityMedium
	case protoapi.Severity_SEVERITY_HIGH:
		return VulnerabilitySeverityHigh
	case protoapi.Severity_SEVERITY_CRITICAL:
		return VulnerabilitySeverityCritical
	default:
		return VulnerabilitySeverityUnknown
	}
}

func suppression(value *protoapi.Suppression) (*VulnerabilitySuppression, error) {
	if value == nil {
		return nil, nil
	}
	var state VulnerabilitySuppressionState
	switch value.GetState() {
	case protoapi.SuppressionState_SUPPRESSION_STATE_NOT_AFFECTED:
		state = VulnerabilitySuppressionStateNotAffected
	case protoapi.SuppressionState_SUPPRESSION_STATE_FALSE_POSITIVE:
		state = VulnerabilitySuppressionStateFalsePositive
	case protoapi.SuppressionState_SUPPRESSION_STATE_ACCEPTED_RISK:
		state = VulnerabilitySuppressionStateAcceptedRisk
	default:
		return nil, fmt.Errorf("invalid v0s suppression state")
	}
	if !value.GetUpdatedAt().IsValid() {
		return nil, fmt.Errorf("invalid v0s suppression timestamp")
	}
	return &VulnerabilitySuppression{
		Team: slug.Slug(value.GetTeam()), Environment: value.GetEnvironment(), Workload: value.GetWorkload(),
		ImageName: value.GetImageName(), PackageKey: value.GetPackageKey(),
		VulnerabilityIdentifier: value.GetVulnerabilityId(), State: state,
		Reason: value.GetDetail(), UpdatedBy: value.GetUpdatedBy(), UpdatedAt: value.GetUpdatedAt().AsTime(),
	}, nil
}

func finding(value *protoapi.Finding) (*VulnerabilityFinding, error) {
	if value.GetWorkload() == nil || value.GetImage() == nil {
		return nil, fmt.Errorf("v0s finding is missing workload or image")
	}
	e, err := epss(value.GetEpss())
	if err != nil {
		return nil, err
	}
	s, err := suppression(value.GetSuppression())
	if err != nil {
		return nil, err
	}
	return &VulnerabilityFinding{
		Workload: &VulnerabilityWorkloadRef{
			Team: slug.Slug(value.GetWorkload().GetTeam()), Environment: value.GetWorkload().GetEnvironment(),
			Name: value.GetWorkload().GetName(),
		},
		Image:                   &VulnerabilityImage{Name: value.GetImage().GetName(), Digest: value.GetImage().GetDigest()},
		VulnerabilityIdentifier: value.GetVulnerabilityId(), Purl: value.GetPurl(), PackageKey: value.GetPackageKey(),
		InstalledVersion: value.GetInstalledVersion(), FixedVersion: optionalString(value.GetFixedVersion()),
		Severity: severity(value.GetSeverity()), SeveritySource: value.GetSeveritySource(), DataSource: value.GetDataSource(),
		Epss: e, KnownExploited: value.GetKnownExploited(), Suppression: s,
	}, nil
}

func advisory(value *protoapi.Vulnerability) (*Vulnerability, error) {
	if value == nil {
		return nil, nil
	}
	e, err := epss(value.GetEpss())
	if err != nil {
		return nil, err
	}
	return &Vulnerability{
		Identifier: value.GetId(), Aliases: value.GetAliases(), Title: value.GetTitle(),
		Description: value.GetDescription(), References: value.GetReferences(),
		Epss: e, KnownExploited: value.GetKnownExploited(),
	}, nil
}

func coverage(value *protoapi.Workload) (*VulnerabilityCoverage, error) {
	result := &VulnerabilityCoverage{Status: VulnerabilityCoverageStatusPending}
	if value == nil {
		return result, nil
	}
	if value.GetImage() == nil {
		if value.GetResolveError() != "" {
			result.Status = VulnerabilityCoverageStatusUnresolvedImage
			result.Error = optionalString(value.GetResolveError())
		}
		return result, nil
	}
	image := value.GetImage()
	result.ImageName = optionalString(image.GetName())
	result.ImageDigest = optionalString(image.GetDigest())
	result.Error = optionalString(image.GetError())
	if image.GetScannedAt() != nil {
		if !image.GetScannedAt().IsValid() {
			return nil, fmt.Errorf("invalid v0s scan timestamp")
		}
		result.ScannedAt = new(image.GetScannedAt().AsTime())
	}
	switch image.GetStatus() {
	case protoapi.ImageStatus_IMAGE_STATUS_SCANNED:
		result.Status = VulnerabilityCoverageStatusScanned
	case protoapi.ImageStatus_IMAGE_STATUS_NO_SBOM:
		result.Status = VulnerabilityCoverageStatusNoSbom
	case protoapi.ImageStatus_IMAGE_STATUS_FAILED:
		result.Status = VulnerabilityCoverageStatusFailed
	}
	return result, nil
}

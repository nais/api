package v0s

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strconv"
	"time"

	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

type Scope struct {
	Team        slug.Slug
	Environment string
	Workload    string
}

type VulnerabilityFindingFilter struct {
	VulnerabilityIdentifiers []string
	Package                  *string
	Severities               []VulnerabilitySeverity
	OnlyWithFix              *bool
	OnlyKnownExploited       *bool
	Suppressed               *SuppressedFilter
}

type VulnerabilityFindingEdge = pagination.Edge[*VulnerabilityFinding]

type VulnerabilityFindingConnection struct {
	pagination.Connection[*VulnerabilityFinding]
	loadFacets func(context.Context) (*VulnerabilityFindingFacets, error)
}

func (c *VulnerabilityFindingConnection) Facets(ctx context.Context) (*VulnerabilityFindingFacets, error) {
	return c.loadFacets(ctx)
}

type VulnerabilityFinding struct {
	Workload                *VulnerabilityWorkloadRef
	Image                   *VulnerabilityImage
	VulnerabilityIdentifier string
	Purl                    string
	PackageKey              string
	InstalledVersion        string
	FixedVersion            *string
	Severity                VulnerabilitySeverity
	SeveritySource          string
	DataSource              string
	Epss                    *VulnerabilityEpss
	KnownExploited          bool
	Suppression             *VulnerabilitySuppression
}

type VulnerabilityWorkloadRef struct {
	Team        slug.Slug
	Environment string
	Name        string
}

type VulnerabilityImage struct {
	Name   string
	Digest string
}

type VulnerabilityFindingFacets struct {
	CountsBySeverity       *VulnerabilitySeverityCounts
	FixableFindings        int64
	KnownExploitedFindings int64
}

type VulnerabilitySeverityCounts struct {
	Unknown, Low, Medium, High, Critical int64
}

type VulnerabilityCoverage struct {
	Status      VulnerabilityCoverageStatus
	ImageName   *string
	ImageDigest *string
	ScannedAt   *time.Time
	Error       *string
}

type Vulnerability struct {
	Identifier     string
	Aliases        []string
	Title          string
	Description    string
	References     []string
	Epss           *VulnerabilityEpss
	KnownExploited bool
}

type VulnerabilityEpss struct {
	Score float64
	Date  scalar.Date
}

type VulnerabilitySuppressionFilter struct {
	VulnerabilityIdentifier *string
	ImageName               *string
}

type VulnerabilitySuppression struct {
	Team                    slug.Slug                     `json:"team"`
	Environment             string                        `json:"environment"`
	Workload                string                        `json:"workload"`
	ImageName               string                        `json:"imageName"`
	PackageKey              string                        `json:"packageKey"`
	VulnerabilityIdentifier string                        `json:"vulnerabilityIdentifier"`
	State                   VulnerabilitySuppressionState `json:"state"`
	Reason                  string                        `json:"reason"`
	UpdatedBy               string                        `json:"updatedBy"`
	UpdatedAt               time.Time                     `json:"updatedAt"`
}

type (
	VulnerabilitySuppressionConnection = pagination.Connection[*VulnerabilitySuppression]
	VulnerabilitySuppressionEdge       = pagination.Edge[*VulnerabilitySuppression]
)

type VulnerabilitySuppressionKeyInput struct {
	Environment             string
	Workload                string
	ImageName               string
	PackageKey              string
	VulnerabilityIdentifier string
}

type NewVulnerabilitySuppressionInput struct {
	Key    *VulnerabilitySuppressionKeyInput
	State  VulnerabilitySuppressionState
	Reason string
}

type SetVulnerabilitySuppressionsInput struct {
	Team  slug.Slug
	Items []*NewVulnerabilitySuppressionInput
}

type DeleteVulnerabilitySuppressionsInput struct {
	Team slug.Slug
	Keys []*VulnerabilitySuppressionKeyInput
}

type SetVulnerabilitySuppressionsPayload struct {
	Suppressions []*VulnerabilitySuppression
}

type DeleteVulnerabilitySuppressionsPayload struct {
	Deleted []*VulnerabilitySuppressionSnapshot
}

type VulnerabilitySeverity string

const (
	VulnerabilitySeverityUnknown  VulnerabilitySeverity = "UNKNOWN"
	VulnerabilitySeverityLow      VulnerabilitySeverity = "LOW"
	VulnerabilitySeverityMedium   VulnerabilitySeverity = "MEDIUM"
	VulnerabilitySeverityHigh     VulnerabilitySeverity = "HIGH"
	VulnerabilitySeverityCritical VulnerabilitySeverity = "CRITICAL"
)

func (v VulnerabilitySeverity) MarshalGQL(w io.Writer) {
	_, _ = io.WriteString(w, strconv.Quote(string(v)))
}

func (v *VulnerabilitySeverity) UnmarshalGQL(value any) error {
	s, err := enumValue(value, "UNKNOWN", "LOW", "MEDIUM", "HIGH", "CRITICAL")
	*v = VulnerabilitySeverity(s)
	return err
}

type SuppressedFilter string

const (
	SuppressedFilterExclude SuppressedFilter = "EXCLUDE"
	SuppressedFilterInclude SuppressedFilter = "INCLUDE"
	SuppressedFilterOnly    SuppressedFilter = "ONLY"
)

func (v SuppressedFilter) MarshalGQL(w io.Writer) { _, _ = io.WriteString(w, strconv.Quote(string(v))) }

func (v *SuppressedFilter) UnmarshalGQL(value any) error {
	s, err := enumValue(value, "EXCLUDE", "INCLUDE", "ONLY")
	*v = SuppressedFilter(s)
	return err
}

type VulnerabilityCoverageStatus string

const (
	VulnerabilityCoverageStatusPending         VulnerabilityCoverageStatus = "PENDING"
	VulnerabilityCoverageStatusScanned         VulnerabilityCoverageStatus = "SCANNED"
	VulnerabilityCoverageStatusNoSbom          VulnerabilityCoverageStatus = "NO_SBOM"
	VulnerabilityCoverageStatusFailed          VulnerabilityCoverageStatus = "FAILED"
	VulnerabilityCoverageStatusUnresolvedImage VulnerabilityCoverageStatus = "UNRESOLVED_IMAGE"
)

func (v VulnerabilityCoverageStatus) MarshalGQL(w io.Writer) {
	_, _ = io.WriteString(w, strconv.Quote(string(v)))
}

func (v *VulnerabilityCoverageStatus) UnmarshalGQL(value any) error {
	s, err := enumValue(value, "PENDING", "SCANNED", "NO_SBOM", "FAILED", "UNRESOLVED_IMAGE")
	*v = VulnerabilityCoverageStatus(s)
	return err
}

type VulnerabilitySuppressionState string

const (
	VulnerabilitySuppressionStateNotAffected   VulnerabilitySuppressionState = "NOT_AFFECTED"
	VulnerabilitySuppressionStateFalsePositive VulnerabilitySuppressionState = "FALSE_POSITIVE"
	VulnerabilitySuppressionStateAcceptedRisk  VulnerabilitySuppressionState = "ACCEPTED_RISK"
)

func (v VulnerabilitySuppressionState) MarshalGQL(w io.Writer) {
	_, _ = io.WriteString(w, strconv.Quote(string(v)))
}

func (v *VulnerabilitySuppressionState) UnmarshalGQL(value any) error {
	s, err := enumValue(value, "NOT_AFFECTED", "FALSE_POSITIVE", "ACCEPTED_RISK")
	*v = VulnerabilitySuppressionState(s)
	return err
}

func enumValue(value any, allowed ...string) (string, error) {
	s, ok := value.(string)
	if !ok || !slices.Contains(allowed, s) {
		return "", fmt.Errorf("invalid enum value")
	}
	return s, nil
}

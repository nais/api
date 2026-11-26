package v0s

import (
	"context"
	"io"
	"strconv"
	"time"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team/teamscope"
	v0sfilter "github.com/nais/api/internal/v0s/filter"
	"github.com/nais/api/internal/workload"
)

type Scope struct {
	Team        slug.Slug
	Environment string
	Workload    string
}

type VulnerabilityFindingFilter = v0sfilter.VulnerabilityFindingFilter

type VulnerabilityWorkloadFilter struct {
	TeamScope    *teamscope.TeamScopeInput
	Environments []string
	Labels       []*model.LabelFilter
}

type VulnerabilityFindingOrder struct {
	Field     VulnerabilityFindingOrderField
	Direction model.OrderDirection
}

type VulnerabilityFindingOrderField string

const (
	VulnerabilityFindingOrderFieldSeverity   VulnerabilityFindingOrderField = "SEVERITY"
	VulnerabilityFindingOrderFieldIdentifier VulnerabilityFindingOrderField = "IDENTIFIER"
)

func (v VulnerabilityFindingOrderField) MarshalGQL(w io.Writer) {
	_, _ = io.WriteString(w, strconv.Quote(string(v)))
}

func (v *VulnerabilityFindingOrderField) UnmarshalGQL(value any) error {
	s, err := enumValue(value, "SEVERITY", "IDENTIFIER")
	*v = VulnerabilityFindingOrderField(s)
	return err
}

type VulnerabilityFindingEdge = pagination.Edge[*VulnerabilityFinding]

type VulnerabilityFindingConnection struct {
	pagination.Connection[*VulnerabilityFinding]
	loadFacets            func(context.Context) (*VulnerabilityFindingFacets, error)
	loadVulnerabilities   func(context.Context, *int, *pagination.Cursor, *VulnerabilityOverviewOrder) (*VulnerabilityOverviewConnection, error)
	loadSummary           func(context.Context) (*VulnerabilitySelectionSummary, error)
	loadAffectedWorkloads func(context.Context, *int, *pagination.Cursor) (*workload.WorkloadConnection, error)
}

func (c *VulnerabilityFindingConnection) AffectedWorkloads(ctx context.Context, first *int, after *pagination.Cursor) (*workload.WorkloadConnection, error) {
	return c.loadAffectedWorkloads(ctx, first, after)
}

func (c *VulnerabilityFindingConnection) Summary(ctx context.Context) (*VulnerabilitySelectionSummary, error) {
	return c.loadSummary(ctx)
}

type VulnerabilitySelectionSummary struct {
	FindingCount                     int64
	UniqueVulnerabilityCount         int64
	AffectedWorkloadCount            int64
	KnownExploitedVulnerabilityCount int64
	KnownExploitedWorkloadCount      int64
	CountsBySeverity                 *VulnerabilitySeverityCounts
	Coverage                         *workload.VulnerabilityCoverageCounts
}

func (c *VulnerabilityFindingConnection) Facets(ctx context.Context) (*VulnerabilityFindingFacets, error) {
	return c.loadFacets(ctx)
}

func (c *VulnerabilityFindingConnection) Vulnerabilities(ctx context.Context, first *int, after *pagination.Cursor, orderBy *VulnerabilityOverviewOrder) (*VulnerabilityOverviewConnection, error) {
	return c.loadVulnerabilities(ctx, first, after, orderBy)
}

type (
	VulnerabilityOverviewConnection = pagination.Connection[*VulnerabilityOverview]
	VulnerabilityOverviewEdge       = pagination.Edge[*VulnerabilityOverview]
)

type VulnerabilityOverview struct {
	VulnerabilityIdentifier string
	Severity                VulnerabilitySeverity
	FindingCount            int64
	AffectedWorkloadCount   int64
}

type VulnerabilityOverviewOrder struct {
	Field     VulnerabilityOverviewOrderField
	Direction model.OrderDirection
}

type VulnerabilityOverviewOrderField string

const (
	VulnerabilityOverviewOrderFieldSeverity              VulnerabilityOverviewOrderField = "SEVERITY"
	VulnerabilityOverviewOrderFieldIdentifier            VulnerabilityOverviewOrderField = "IDENTIFIER"
	VulnerabilityOverviewOrderFieldAffectedWorkloadCount VulnerabilityOverviewOrderField = "AFFECTED_WORKLOAD_COUNT"
)

func (v VulnerabilityOverviewOrderField) MarshalGQL(w io.Writer) {
	_, _ = io.WriteString(w, strconv.Quote(string(v)))
}

func (v *VulnerabilityOverviewOrderField) UnmarshalGQL(value any) error {
	s, err := enumValue(value, "SEVERITY", "IDENTIFIER", "AFFECTED_WORKLOAD_COUNT")
	*v = VulnerabilityOverviewOrderField(s)
	return err
}

type VulnerabilityWorkloadOverview struct {
	FindingCount                     int64
	UniqueVulnerabilityCount         int64
	KnownExploitedVulnerabilityCount int64
	CountsBySeverity                 *VulnerabilitySeverityCounts
	Coverage                         *VulnerabilityCoverage
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
	PackageTypes           []model.StringFacetItem
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

type VulnerabilitySeverity = v0sfilter.VulnerabilitySeverity

const (
	VulnerabilitySeverityUnknown  = v0sfilter.VulnerabilitySeverityUnknown
	VulnerabilitySeverityLow      = v0sfilter.VulnerabilitySeverityLow
	VulnerabilitySeverityMedium   = v0sfilter.VulnerabilitySeverityMedium
	VulnerabilitySeverityHigh     = v0sfilter.VulnerabilitySeverityHigh
	VulnerabilitySeverityCritical = v0sfilter.VulnerabilitySeverityCritical
)

type SuppressedFilter = v0sfilter.SuppressedFilter

const (
	SuppressedFilterExclude = v0sfilter.SuppressedFilterExclude
	SuppressedFilterInclude = v0sfilter.SuppressedFilterInclude
	SuppressedFilterOnly    = v0sfilter.SuppressedFilterOnly
)

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
	return v0sfilter.EnumValue(value, allowed...)
}

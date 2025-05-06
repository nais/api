package maintenance

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
)

type (
	WorkloadVulnerabilitySummaryConnection      = pagination.Connection[*WorkloadVulnerabilitySummary]
	WorkloadVulnerabilitySummaryEdge            = pagination.Edge[*WorkloadVulnerabilitySummary]
	ImageVulnerabilityConnection                = pagination.Connection[*ImageVulnerability]
	ImageVulnerabilityEdge                      = pagination.Edge[*ImageVulnerability]
	ContainerImageWorkloadReferenceConnection   = pagination.Connection[*ContainerImageWorkloadReference]
	ContainerImageWorkloadReferenceEdge         = pagination.Edge[*ContainerImageWorkloadReference]
	ImageVulnerabilityAnalysisCommentConnection = pagination.Connection[*ImageVulnerabilityAnalysisComment]
	ImageVulnerabilityAnalysisCommentEdge       = pagination.Edge[*ImageVulnerabilityAnalysisComment]
)

type ContainerImageWorkloadReference struct {
	Reference       *workload.Reference `json:"-"`
	TeamSlug        slug.Slug           `json:"-"`
	EnvironmentName string              `json:"-"`
}

type ImageVulnerability struct {
	Identifier      string                           `json:"identifier"`
	Severity        ImageVulnerabilitySeverity       `json:"severity"`
	Description     string                           `json:"description"`
	Package         string                           `json:"package"`
	State           ImageVulnerabilityState          `json:"state"`
	AnalysisTrail   *ImageVulnerabilityAnalysisTrail `json:"analysisTrail"`
	vulnerabilityID string                           `json:"-"`
}

func (ImageVulnerability) IsNode() {}
func (i *ImageVulnerability) ID() ident.Ident {
	return ident.NewIdent(identVulnerability, i.vulnerabilityID)
}

type ImageVulnerabilityOrder struct {
	Field     ImageVulnerabilityOrderField `json:"field"`
	Direction model.OrderDirection         `json:"direction"`
}

type ImageVulnerabilitySummary struct {
	Total      int `json:"total"`
	RiskScore  int `json:"riskScore"`
	Low        int `json:"low"`
	Medium     int `json:"medium"`
	High       int `json:"high"`
	Critical   int `json:"critical"`
	Unassigned int `json:"unassigned"`
}

type ImageVulnerabilityOrderField string

func (e ImageVulnerabilityOrderField) IsValid() bool {
	_, ok := SortFilterImageVulnerabilities[e]
	return ok
}

func (e ImageVulnerabilityOrderField) String() string {
	return string(e)
}

func (e *ImageVulnerabilityOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ImageVulnerabilityOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ImageVulnerabilityOrderField", str)
	}
	return nil
}

func (e ImageVulnerabilityOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type ImageVulnerabilitySeverity string

const (
	ImageVulnerabilitySeverityLow        ImageVulnerabilitySeverity = "LOW"
	ImageVulnerabilitySeverityMedium     ImageVulnerabilitySeverity = "MEDIUM"
	ImageVulnerabilitySeverityHigh       ImageVulnerabilitySeverity = "HIGH"
	ImageVulnerabilitySeverityCritical   ImageVulnerabilitySeverity = "CRITICAL"
	ImageVulnerabilitySeverityUnassigned ImageVulnerabilitySeverity = "UNASSIGNED"
)

func (e ImageVulnerabilitySeverity) IsValid() bool {
	switch e {
	case ImageVulnerabilitySeverityLow, ImageVulnerabilitySeverityMedium, ImageVulnerabilitySeverityHigh, ImageVulnerabilitySeverityCritical, ImageVulnerabilitySeverityUnassigned:
		return true
	}
	return false
}

func (e ImageVulnerabilitySeverity) String() string {
	return string(e)
}

func (e *ImageVulnerabilitySeverity) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ImageVulnerabilitySeverity(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ImageVulnerabilitySeverity", str)
	}
	return nil
}

func (e ImageVulnerabilitySeverity) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type ImageVulnerabilityState string

const (
	ImageVulnerabilityStateTriage        ImageVulnerabilityState = "TRIAGE"
	ImageVulnerabilityStateResolved      ImageVulnerabilityState = "RESOLVED"
	ImageVulnerabilityStateFalsePositive ImageVulnerabilityState = "FALSE_POSITIVE"
	ImageVulnerabilityStateNotAffected   ImageVulnerabilityState = "NOT_AFFECTED"
)

var AllImageVulnerabilityState = []ImageVulnerabilityState{
	ImageVulnerabilityStateTriage,
	ImageVulnerabilityStateResolved,
	ImageVulnerabilityStateFalsePositive,
	ImageVulnerabilityStateNotAffected,
}

func (e ImageVulnerabilityState) IsValid() bool {
	switch e {
	case ImageVulnerabilityStateTriage, ImageVulnerabilityStateResolved, ImageVulnerabilityStateFalsePositive, ImageVulnerabilityStateNotAffected:
		return true
	}
	return false
}

func (e ImageVulnerabilityState) String() string {
	return string(e)
}

func (e *ImageVulnerabilityState) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ImageVulnerabilityState(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ImageVulnerabilityState", str)
	}
	return nil
}

func (e ImageVulnerabilityState) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type ImageDetails struct {
	Name               string                     `json:"name"`
	Version            string                     `json:"version"`
	Summary            *ImageVulnerabilitySummary `json:"summary"`
	HasSBOM            bool                       `json:"hasSbom"`
	ProjectURL         string                     `json:"projectUrl"`
	WorkloadReferences []*WorkloadReference       `json:"-"`
}

type WorkloadReference struct {
	Environment  string `json:"environment"`
	Team         string `json:"team"`
	WorkloadType string `json:"workloadType"`
	Name         string `json:"name"`
}

func toGraphWorkloadReference(r *WorkloadReference) *ContainerImageWorkloadReference {
	workloadType := workload.TypeApplication

	if r.WorkloadType == "job" {
		workloadType = workload.TypeJob
	}

	return &ContainerImageWorkloadReference{
		Reference: &workload.Reference{
			Name: r.Name,
			Type: workloadType,
		},
		TeamSlug:        slug.Slug(r.Team),
		EnvironmentName: r.Environment,
	}
}

type TeamVulnerabilitySummary struct {
	RiskScore  int     `json:"riskScore"`
	Critical   int     `json:"critical"`
	High       int     `json:"high"`
	Medium     int     `json:"medium"`
	Low        int     `json:"low"`
	Unassigned int     `json:"unassigned"`
	BomCount   int     `json:"bomCount"`
	Coverage   float64 `json:"coverage"`

	TeamSlug slug.Slug `json:"-"`
}

type WorkloadVulnerabilitySummary struct {
	// The workload.
	// Workload workload.Workload `json:"workload"`
	HasSbom bool `json:"hasSBOM"`
	// The vulnerability summary for the workload.
	Summary           *ImageVulnerabilitySummary `json:"summary"`
	TeamSlug          slug.Slug                  `json:"-"`
	EnvironmentName   string                     `json:"-"`
	WorkloadReference *workload.Reference        `json:"-"`
}

func (w WorkloadVulnerabilitySummary) IsNode() {}
func (w *WorkloadVulnerabilitySummary) ID() ident.Ident {
	return newWorkloadVulnerabilitySummaryIdent(WorkloadReference{
		Environment:  w.EnvironmentName,
		Team:         w.TeamSlug.String(),
		Name:         w.WorkloadReference.Name,
		WorkloadType: w.WorkloadReference.Type.String(),
	})
}

// Ordering options when fetching vulnerability summaries for workloads.
type VulnerabilitySummaryOrder struct {
	// The field to order items by.
	Field VulnerabilitySummaryOrderByField `json:"field"`
	// The direction to order items by.
	Direction model.OrderDirection `json:"direction"`
}

type VulnerabilitySummaryOrderByField string

const (
	// Order by name.
	VulnerabilitySummaryOrderByFieldName VulnerabilitySummaryOrderByField = "NAME"
	// Order by the name of the environment the workload is deployed in.
	VulnerabilitySummaryOrderByFieldEnvironment VulnerabilitySummaryOrderByField = "ENVIRONMENT"
	// Order by risk score"
	VulnerabilitySummaryOrderByFieldVulnerabilityRiskScore VulnerabilitySummaryOrderByField = "VULNERABILITY_RISK_SCORE"
	// Order by vulnerability severity critical"
	VulnerabilitySummaryOrderByFieldVulnerabilitySeverityCritical VulnerabilitySummaryOrderByField = "VULNERABILITY_SEVERITY_CRITICAL"
	// Order by vulnerability severity high"
	VulnerabilitySummaryOrderByFieldVulnerabilitySeverityHigh VulnerabilitySummaryOrderByField = "VULNERABILITY_SEVERITY_HIGH"
	// Order by vulnerability severity medium"
	VulnerabilitySummaryOrderByFieldVulnerabilitySeverityMedium VulnerabilitySummaryOrderByField = "VULNERABILITY_SEVERITY_MEDIUM"
	// Order by vulnerability severity low"
	VulnerabilitySummaryOrderByFieldVulnerabilitySeverityLow VulnerabilitySummaryOrderByField = "VULNERABILITY_SEVERITY_LOW"
	// Order by vulnerability severity unassigned"
	VulnerabilitySummaryOrderByFieldVulnerabilitySeverityUnassigned VulnerabilitySummaryOrderByField = "VULNERABILITY_SEVERITY_UNASSIGNED"
)

var AllVulnerabilitySummaryOrderByField = []VulnerabilitySummaryOrderByField{
	VulnerabilitySummaryOrderByFieldName,
	VulnerabilitySummaryOrderByFieldEnvironment,
	VulnerabilitySummaryOrderByFieldVulnerabilityRiskScore,
	VulnerabilitySummaryOrderByFieldVulnerabilitySeverityCritical,
	VulnerabilitySummaryOrderByFieldVulnerabilitySeverityHigh,
	VulnerabilitySummaryOrderByFieldVulnerabilitySeverityMedium,
	VulnerabilitySummaryOrderByFieldVulnerabilitySeverityLow,
	VulnerabilitySummaryOrderByFieldVulnerabilitySeverityUnassigned,
}

func (e VulnerabilitySummaryOrderByField) IsValid() bool {
	switch e {
	case VulnerabilitySummaryOrderByFieldName, VulnerabilitySummaryOrderByFieldEnvironment, VulnerabilitySummaryOrderByFieldVulnerabilityRiskScore, VulnerabilitySummaryOrderByFieldVulnerabilitySeverityCritical, VulnerabilitySummaryOrderByFieldVulnerabilitySeverityHigh, VulnerabilitySummaryOrderByFieldVulnerabilitySeverityMedium, VulnerabilitySummaryOrderByFieldVulnerabilitySeverityLow, VulnerabilitySummaryOrderByFieldVulnerabilitySeverityUnassigned:
		return true
	}
	return false
}

func (e VulnerabilitySummaryOrderByField) String() string {
	return string(e)
}

func (e *VulnerabilitySummaryOrderByField) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = VulnerabilitySummaryOrderByField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid VulnerabilitySummaryOrderByField", str)
	}
	return nil
}

func (e VulnerabilitySummaryOrderByField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type TeamVulnerabilityRanking string

const (
	// Top third most vulnerable teams.
	TeamVulnerabilityRankingMostVulnerable TeamVulnerabilityRanking = "MOST_VULNERABLE"
	// Middle third most vulnerable teams.
	TeamVulnerabilityRankingMiddle TeamVulnerabilityRanking = "MIDDLE"
	// Bottom third most vulnerable teams.
	TeamVulnerabilityRankingLeastVulnerable TeamVulnerabilityRanking = "LEAST_VULNERABLE"
	// Unknown ranking.
	TeamVulnerabilityRankingUnknown TeamVulnerabilityRanking = "UNKNOWN"
)

var AllTeamVulnerabilityRanking = []TeamVulnerabilityRanking{
	TeamVulnerabilityRankingMostVulnerable,
	TeamVulnerabilityRankingMiddle,
	TeamVulnerabilityRankingLeastVulnerable,
	TeamVulnerabilityRankingUnknown,
}

func (e TeamVulnerabilityRanking) IsValid() bool {
	switch e {
	case TeamVulnerabilityRankingMostVulnerable, TeamVulnerabilityRankingMiddle, TeamVulnerabilityRankingLeastVulnerable, TeamVulnerabilityRankingUnknown:
		return true
	}
	return false
}

func (e TeamVulnerabilityRanking) String() string {
	return string(e)
}

func (e *TeamVulnerabilityRanking) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = TeamVulnerabilityRanking(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid TeamVulnerabilityRanking", str)
	}
	return nil
}

func (e TeamVulnerabilityRanking) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type TeamVulnerabilityRiskScoreTrend string

const (
	// Risk score is increasing.
	TeamVulnerabilityRiskScoreTrendUp TeamVulnerabilityRiskScoreTrend = "UP"
	// Risk score is decreasing.
	TeamVulnerabilityRiskScoreTrendDown TeamVulnerabilityRiskScoreTrend = "DOWN"
	// Risk score is not changing.
	TeamVulnerabilityRiskScoreTrendFlat TeamVulnerabilityRiskScoreTrend = "FLAT"
)

var AllTeamVulnerabilityRiskScoreTrend = []TeamVulnerabilityRiskScoreTrend{
	TeamVulnerabilityRiskScoreTrendUp,
	TeamVulnerabilityRiskScoreTrendDown,
	TeamVulnerabilityRiskScoreTrendFlat,
}

func (e TeamVulnerabilityRiskScoreTrend) IsValid() bool {
	switch e {
	case TeamVulnerabilityRiskScoreTrendUp, TeamVulnerabilityRiskScoreTrendDown, TeamVulnerabilityRiskScoreTrendFlat:
		return true
	}
	return false
}

func (e TeamVulnerabilityRiskScoreTrend) String() string {
	return string(e)
}

func (e *TeamVulnerabilityRiskScoreTrend) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = TeamVulnerabilityRiskScoreTrend(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid TeamVulnerabilityRiskScoreTrend", str)
	}
	return nil
}

func (e TeamVulnerabilityRiskScoreTrend) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type ImageVulnerabilityAnalysisComment struct {
	Comment    string                          `json:"comment"`
	State      ImageVulnerabilityAnalysisState `json:"state"`
	Suppressed bool                            `json:"suppressed"`
	Timestamp  time.Time                       `json:"timestamp"`
	OnBehalfOf string                          `json:"onBehalfOf"`
}

type ImageVulnerabilityAnalysisTrail struct {
	State      ImageVulnerabilityAnalysisState `json:"state"`
	Suppressed bool                            `json:"suppressed"`

	AllComments []*ImageVulnerabilityAnalysisComment `json:"-"`
}

type UpdateImageVulnerabilityInput struct {
	// The id of the vulnerability to suppress.
	VulnerabilityID ident.Ident `json:"vulnerabilityID"`
	// The analysis state of the vulnerability.
	AnalysisState ImageVulnerabilityAnalysisState `json:"analysisState"`
	// The a comment for suppressing the vulnerability.
	Comment string `json:"comment"`
	// Should the vulnerability be suppressed.
	Suppress bool `json:"suppress"`
}

type UpdateImageVulnerabilityPayload struct {
	Vulnerability *ImageVulnerability `json:"vulnerability"`
}

type ImageVulnerabilityAnalysisState string

const (
	// Vulnerability is triaged.
	ImageVulnerabilityAnalysisStateInTriage ImageVulnerabilityAnalysisState = "IN_TRIAGE"
	// Vulnerability is resolved.
	ImageVulnerabilityAnalysisStateResolved ImageVulnerabilityAnalysisState = "RESOLVED"
	// Vulnerability is marked as false positive.
	ImageVulnerabilityAnalysisStateFalsePositive ImageVulnerabilityAnalysisState = "FALSE_POSITIVE"
	// Vulnerability is marked as not affected.
	ImageVulnerabilityAnalysisStateNotAffected ImageVulnerabilityAnalysisState = "NOT_AFFECTED"
)

var AllImageVulnerabilityAnalysisState = []ImageVulnerabilityAnalysisState{
	ImageVulnerabilityAnalysisStateInTriage,
	ImageVulnerabilityAnalysisStateResolved,
	ImageVulnerabilityAnalysisStateFalsePositive,
	ImageVulnerabilityAnalysisStateNotAffected,
}

func (e ImageVulnerabilityAnalysisState) IsValid() bool {
	switch e {
	case ImageVulnerabilityAnalysisStateInTriage, ImageVulnerabilityAnalysisStateResolved, ImageVulnerabilityAnalysisStateFalsePositive, ImageVulnerabilityAnalysisStateNotAffected:
		return true
	}
	return false
}

func (e ImageVulnerabilityAnalysisState) String() string {
	return string(e)
}

func (e *ImageVulnerabilityAnalysisState) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ImageVulnerabilityAnalysisState(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ImageVulnerabilityAnalysisState", str)
	}
	return nil
}

func (e ImageVulnerabilityAnalysisState) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type TeamVulnerabilityStatus struct {
	State       TeamVulnerabilityState `json:"state"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
}

type TeamVulnerabilityState string

const (
	TeamVulnerabilityStateOk                         TeamVulnerabilityState = "OK"
	TeamVulnerabilityStateTooManyVulnerableWorkloads TeamVulnerabilityState = "TOO_MANY_VULNERABLE_WORKLOADS"
	TeamVulnerabilityStateCoverageTooLow             TeamVulnerabilityState = "COVERAGE_TOO_LOW"
	TeamVulnerabilityStateVulnerable                 TeamVulnerabilityState = "VULNERABLE"
	TeamVulnerabilityStateMissingSbom                TeamVulnerabilityState = "MISSING_SBOM"
)

var AllTeamVulnerabilityState = []TeamVulnerabilityState{
	TeamVulnerabilityStateOk,
	TeamVulnerabilityStateTooManyVulnerableWorkloads,
	TeamVulnerabilityStateCoverageTooLow,
	TeamVulnerabilityStateVulnerable,
	TeamVulnerabilityStateMissingSbom,
}

func (e TeamVulnerabilityState) IsValid() bool {
	switch e {
	case TeamVulnerabilityStateOk, TeamVulnerabilityStateTooManyVulnerableWorkloads, TeamVulnerabilityStateCoverageTooLow, TeamVulnerabilityStateVulnerable, TeamVulnerabilityStateMissingSbom:
		return true
	}
	return false
}

func (e TeamVulnerabilityState) String() string {
	return string(e)
}

func (e *TeamVulnerabilityState) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = TeamVulnerabilityState(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid TeamVulnerabilityState", str)
	}
	return nil
}

func (e TeamVulnerabilityState) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type TeamVulnerabilitySummaryFilter struct {
	Environments []string `json:"environments,omitempty"`
}

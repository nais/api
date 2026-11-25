package filter

import (
	"fmt"
	"io"
	"slices"
	"strconv"
)

type VulnerabilityFindingFilter struct {
	VulnerabilityIdentifiers []string
	Package                  *string
	PackageType              *string
	Severities               []VulnerabilitySeverity
	OnlyWithFix              *bool
	OnlyKnownExploited       *bool
	Suppressed               *SuppressedFilter
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
	s, err := EnumValue(value, "UNKNOWN", "LOW", "MEDIUM", "HIGH", "CRITICAL")
	*v = VulnerabilitySeverity(s)
	return err
}

type SuppressedFilter string

const (
	SuppressedFilterExclude SuppressedFilter = "EXCLUDE"
	SuppressedFilterInclude SuppressedFilter = "INCLUDE"
	SuppressedFilterOnly    SuppressedFilter = "ONLY"
)

func (v SuppressedFilter) MarshalGQL(w io.Writer) {
	_, _ = io.WriteString(w, strconv.Quote(string(v)))
}

func (v *SuppressedFilter) UnmarshalGQL(value any) error {
	s, err := EnumValue(value, "EXCLUDE", "INCLUDE", "ONLY")
	*v = SuppressedFilter(s)
	return err
}

func EnumValue(value any, allowed ...string) (string, error) {
	s, ok := value.(string)
	if !ok || !slices.Contains(allowed, s) {
		return "", fmt.Errorf("invalid enum value")
	}
	return s, nil
}

package v0s

import (
	"context"
	"net/url"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
)

type identType int

const (
	identFinding identType = iota
	identAdvisory
	identSuppression
)

func init() {
	ident.RegisterIdentType(identFinding, "VFI", getFindingByIdent)
	ident.RegisterIdentType(identAdvisory, "VAD", getAdvisoryByIdent)
	ident.RegisterIdentType(identSuppression, "VAS", getSuppressionByIdent)
}

func newIdent(kind identType, parts ...string) ident.Ident {
	for i, part := range parts {
		parts[i] = url.QueryEscape(part)
	}
	return ident.NewIdent(kind, parts...)
}

func identParts(id ident.Ident, count int) ([]string, error) {
	parts := id.Parts()
	if len(parts) != count {
		return nil, apierror.Errorf("Invalid vulnerability node ID.")
	}
	for i, part := range parts {
		value, err := url.QueryUnescape(part)
		if err != nil {
			return nil, apierror.Errorf("Invalid vulnerability node ID.")
		}
		parts[i] = value
	}
	return parts, nil
}

func (*VulnerabilityFinding) IsNode() {}

func (f *VulnerabilityFinding) ID() ident.Ident {
	return newIdent(identFinding, f.Workload.Environment, f.Workload.Team.String(), f.Workload.Name,
		f.Image.Name, f.Image.Digest, f.VulnerabilityIdentifier, f.Purl)
}

func (*Vulnerability) IsNode() {}

func (v *Vulnerability) ID() ident.Ident {
	return newIdent(identAdvisory, v.Identifier)
}

func (*VulnerabilitySuppression) IsNode() {}

func (s *VulnerabilitySuppression) ID() ident.Ident {
	return newIdent(identSuppression, s.Team.String(), s.Environment, s.Workload, s.ImageName, s.PackageKey, s.VulnerabilityIdentifier)
}

func getAdvisoryByIdent(ctx context.Context, id ident.Ident) (*Vulnerability, error) {
	parts, err := identParts(id, 1)
	if err != nil {
		return nil, err
	}
	value, err := VulnerabilityByIdentifier(ctx, parts[0])
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, apierror.ErrNotFound
	}
	return value, nil
}

func getFindingByIdent(ctx context.Context, id ident.Ident) (*VulnerabilityFinding, error) {
	parts, err := identParts(id, 7)
	if err != nil {
		return nil, err
	}
	input := Scope{Environment: parts[0], Team: slug.Slug(parts[1]), Workload: parts[2]}
	filter := &VulnerabilityFindingFilter{
		VulnerabilityIdentifiers: []string{parts[5]}, Package: &parts[6], Suppressed: new(SuppressedFilterInclude),
	}
	var after *pagination.Cursor
	for {
		result, err := Findings(ctx, input, filter, new(maxPageSize), after, nil, nil, nil)
		if err != nil {
			return nil, err
		}
		for _, value := range result.Nodes() {
			if value.ID() == id {
				return value, nil
			}
		}
		if !result.PageInfo.HasNextPage {
			return nil, apierror.ErrNotFound
		}
		after = result.PageInfo.EndCursor
	}
}

func getSuppressionByIdent(ctx context.Context, id ident.Ident) (*VulnerabilitySuppression, error) {
	parts, err := identParts(id, 6)
	if err != nil {
		return nil, err
	}
	input := Scope{Team: slug.Slug(parts[0]), Environment: parts[1], Workload: parts[2]}
	filter := &VulnerabilitySuppressionFilter{ImageName: &parts[3], VulnerabilityIdentifier: &parts[5]}
	var after *pagination.Cursor
	for {
		result, err := Suppressions(ctx, input, filter, new(maxPageSize), after, nil, nil)
		if err != nil {
			return nil, err
		}
		for _, value := range result.Nodes() {
			if value.ID() == id {
				return value, nil
			}
		}
		if !result.PageInfo.HasNextPage {
			return nil, apierror.ErrNotFound
		}
		after = result.PageInfo.EndCursor
	}
}

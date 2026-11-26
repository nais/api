package v0s

import (
	"context"
	"strings"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/slug"
	protoapi "github.com/nais/v0s/api/v0s/v1"
)

func writeManager(ctx context.Context, team slug.Slug) (*Manager, error) {
	if err := requireRead(ctx); err != nil {
		return nil, err
	}
	if err := team.Validate(); err != nil {
		return nil, err
	}
	m, _ := ctx.Value(managerKey).(*Manager)
	// TODO: Remove the migration gate, V0S_SUPPRESSIONS_ENABLED, and references
	// to updateImageVulnerability once all tenants write exclusively to v0s and
	// the v13s suppression sync is stopped. Keep configuration and team authorization checks.
	if m == nil || !m.SuppressionsEnabled {
		return nil, apierror.Errorf("Assessment writes are disabled during migration; use updateImageVulnerability.")
	}
	if m.Suppressions == nil {
		return nil, apierror.Errorf("Assessment writes are not configured.")
	}
	if err := authz.CanUpdateVulnerability(ctx, team); err != nil {
		return nil, err
	}
	return m, nil
}

func suppressionKey(input *VulnerabilitySuppressionKeyInput) (*protoapi.SuppressionKey, error) {
	if input == nil || strings.TrimSpace(input.Environment) == "" ||
		strings.TrimSpace(input.Workload) == "" || strings.TrimSpace(input.ImageName) == "" ||
		strings.TrimSpace(input.PackageKey) == "" || strings.TrimSpace(input.VulnerabilityIdentifier) == "" {
		return nil, apierror.Errorf("All suppression key fields must be non-empty")
	}
	return &protoapi.SuppressionKey{
		Environment: input.Environment, Workload: input.Workload, ImageName: input.ImageName,
		Package: input.PackageKey, VulnerabilityId: input.VulnerabilityIdentifier,
	}, nil
}

func SetSuppressions(ctx context.Context, input SetVulnerabilitySuppressionsInput) (*SetVulnerabilitySuppressionsPayload, error) {
	if len(input.Items) == 0 {
		return nil, apierror.Errorf("items cannot be empty")
	}
	m, err := writeManager(ctx, input.Team)
	if err != nil {
		return nil, err
	}
	req := &protoapi.SetSuppressionsRequest{
		Team: input.Team.String(), Actor: authz.ActorFromContext(ctx).User.Identity(),
	}
	for _, item := range input.Items {
		if item == nil {
			return nil, apierror.Errorf("suppression item cannot be null")
		}
		if strings.TrimSpace(item.Reason) == "" {
			return nil, apierror.Errorf("reason cannot be empty")
		}
		key, err := suppressionKey(item.Key)
		if err != nil {
			return nil, err
		}
		var state protoapi.SuppressionState
		switch item.State {
		case VulnerabilitySuppressionStateNotAffected:
			state = protoapi.SuppressionState_SUPPRESSION_STATE_NOT_AFFECTED
		case VulnerabilitySuppressionStateFalsePositive:
			state = protoapi.SuppressionState_SUPPRESSION_STATE_FALSE_POSITIVE
		case VulnerabilitySuppressionStateAcceptedRisk:
			state = protoapi.SuppressionState_SUPPRESSION_STATE_ACCEPTED_RISK
		default:
			return nil, apierror.Errorf("invalid suppression state")
		}
		req.Suppressions = append(req.Suppressions, &protoapi.NewSuppression{Key: key, State: state, Detail: item.Reason})
	}
	l, err := fromContext(ctx)
	if err != nil {
		return nil, err
	}
	inventory, err := l.inventory.Load(ctx, struct{}{})
	if err != nil {
		return nil, err
	}
	for _, item := range req.Suppressions {
		key := item.GetKey()
		if _, exists := inventory[workloadKey{key.GetEnvironment(), req.Team, key.GetWorkload()}]; !exists {
			return nil, apierror.Errorf("Cannot assess a workload that does not exist.")
		}
	}
	resp, err := m.Suppressions.SetSuppressions(ctx, req)
	if err != nil {
		return nil, backendError("set v0s suppressions", err)
	}
	result := &SetVulnerabilitySuppressionsPayload{Suppressions: make([]*VulnerabilitySuppression, 0, len(resp.GetSuppressions()))}
	for _, value := range resp.GetSuppressions() {
		s, err := suppression(value)
		if err != nil {
			return nil, err
		}
		recordAssessment(ctx, activitylog.ActivityLogEntryActionUpdated, s)
		result.Suppressions = append(result.Suppressions, s)
	}
	return result, nil
}

func DeleteSuppressions(ctx context.Context, input DeleteVulnerabilitySuppressionsInput) (*DeleteVulnerabilitySuppressionsPayload, error) {
	if len(input.Keys) == 0 {
		return nil, apierror.Errorf("keys cannot be empty")
	}
	m, err := writeManager(ctx, input.Team)
	if err != nil {
		return nil, err
	}
	req := &protoapi.DeleteSuppressionsRequest{Team: input.Team.String()}
	for _, input := range input.Keys {
		key, err := suppressionKey(input)
		if err != nil {
			return nil, err
		}
		req.Keys = append(req.Keys, key)
	}
	resp, err := m.Suppressions.DeleteSuppressions(ctx, req)
	if err != nil {
		return nil, backendError("delete v0s suppressions", err)
	}
	result := &DeleteVulnerabilitySuppressionsPayload{Deleted: make([]*VulnerabilitySuppressionSnapshot, 0, len(resp.GetDeleted()))}
	for _, value := range resp.GetDeleted() {
		s, err := suppression(value)
		if err != nil {
			return nil, err
		}
		recordAssessment(ctx, activitylog.ActivityLogEntryActionDeleted, s)
		result.Deleted = append(result.Deleted, assessmentSnapshot(s))
	}
	return result, nil
}

package v0s

import (
	"context"
	"fmt"
	"time"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/slug"
)

const activityLogResourceType activitylog.ActivityLogEntryResourceType = "VULNERABILITY_SUPPRESSION"

func init() {
	activitylog.RegisterTransformer(activityLogResourceType, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		data, err := activitylog.UnmarshalData[VulnerabilitySuppressionActivityLogEntryData](entry)
		if err != nil {
			return nil, err
		}
		var message string
		switch entry.Action {
		case activitylog.ActivityLogEntryActionUpdated:
			message = "Updated vulnerability assessment"
		case activitylog.ActivityLogEntryActionDeleted:
			message = "Deleted vulnerability assessment"
		default:
			return nil, fmt.Errorf("unsupported vulnerability assessment activity: %q", entry.Action)
		}
		return VulnerabilitySuppressionActivityLogEntry{
			GenericActivityLogEntry: entry.WithMessage(message), Data: data,
		}, nil
	})
	activitylog.RegisterActivityType("VULNERABILITY_SUPPRESSION_UPDATED", activitylog.ActivityLogEntryActionUpdated,
		activityLogResourceType, "A workload vulnerability assessment was set.", activitylog.IgnoreWebhook())
	activitylog.RegisterActivityType("VULNERABILITY_SUPPRESSION_DELETED", activitylog.ActivityLogEntryActionDeleted,
		activityLogResourceType, "A workload vulnerability assessment was deleted.", activitylog.IgnoreWebhook())
}

type VulnerabilitySuppressionActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *VulnerabilitySuppressionActivityLogEntryData `json:"data"`
}

type VulnerabilitySuppressionActivityLogEntryData struct {
	Assessment *VulnerabilitySuppressionSnapshot `json:"assessment"`
}

type VulnerabilitySuppressionSnapshot struct {
	TeamSlug                slug.Slug                     `json:"team"`
	EnvironmentName         string                        `json:"environment"`
	WorkloadName            string                        `json:"workload"`
	ImageName               string                        `json:"imageName"`
	PackageKey              string                        `json:"packageKey"`
	VulnerabilityIdentifier string                        `json:"vulnerabilityIdentifier"`
	State                   VulnerabilitySuppressionState `json:"state"`
	Reason                  string                        `json:"reason"`
	UpdatedBy               string                        `json:"updatedBy"`
	UpdatedAt               time.Time                     `json:"updatedAt"`
}

func recordAssessment(ctx context.Context, action activitylog.ActivityLogEntryAction, assessment *VulnerabilitySuppression) error {
	return activitylog.Create(ctx, activitylog.CreateInput{
		Action: action, Actor: authz.ActorFromContext(ctx).User,
		ResourceType: activityLogResourceType, ResourceName: assessment.Workload,
		TeamSlug: &assessment.Team, EnvironmentName: &assessment.Environment,
		Data: VulnerabilitySuppressionActivityLogEntryData{Assessment: assessmentSnapshot(assessment)},
	})
}

func assessmentSnapshot(assessment *VulnerabilitySuppression) *VulnerabilitySuppressionSnapshot {
	return &VulnerabilitySuppressionSnapshot{
		TeamSlug: assessment.Team, EnvironmentName: assessment.Environment, WorkloadName: assessment.Workload,
		ImageName: assessment.ImageName, PackageKey: assessment.PackageKey, VulnerabilityIdentifier: assessment.VulnerabilityIdentifier,
		State: assessment.State, Reason: assessment.Reason, UpdatedBy: assessment.UpdatedBy, UpdatedAt: assessment.UpdatedAt,
	}
}

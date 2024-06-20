package auditevent

import "github.com/nais/api/internal/graph/model"

type AuditEventTeamSetAlertsSlackChannel struct {
	BaseTeamAuditEvent
	Data model.AuditEventTeamSetAlertsSlackChannelData
}

func (a AuditEventTeamSetAlertsSlackChannel) GetData() any {
	return a.Data
}

type AuditEventTeamSetDefaultSlackChannel struct {
	BaseTeamAuditEvent
	Data model.AuditEventTeamSetDefaultSlackChannelData
}

func (a AuditEventTeamSetDefaultSlackChannel) GetData() any {
	return a.Data
}

type AuditEventTeamSetPurpose struct {
	BaseTeamAuditEvent
	Data model.AuditEventTeamSetPurposeData
}

func (a AuditEventTeamSetPurpose) GetData() any {
	return a.Data
}

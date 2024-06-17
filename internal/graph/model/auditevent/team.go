package auditevent

import "github.com/nais/api/internal/graph/model"

type AuditEventTeamSetAlertsSlackChannel struct {
	BaseAuditEvent
	Data model.AuditEventTeamSetAlertsSlackChannelData
}

func (a AuditEventTeamSetAlertsSlackChannel) GetData() any {
	return a.Data
}

type AuditEventTeamSetDefaultSlackChannel struct {
	BaseAuditEvent
	Data model.AuditEventTeamSetDefaultSlackChannelData
}

func (a AuditEventTeamSetDefaultSlackChannel) GetData() any {
	return a.Data
}

type AuditEventTeamSetPurpose struct {
	BaseAuditEvent
	Data model.AuditEventTeamSetPurposeData
}

func (a AuditEventTeamSetPurpose) GetData() any {
	return a.Data
}

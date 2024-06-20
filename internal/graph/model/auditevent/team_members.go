package auditevent

import (
	"github.com/nais/api/internal/graph/model"
)

type AuditEventMemberAdded struct {
	BaseTeamAuditEvent
	Data model.AuditEventMemberAddedData
}

func (a AuditEventMemberAdded) GetData() any {
	return a.Data
}

type AuditEventMemberRemoved struct {
	BaseTeamAuditEvent
	Data model.AuditEventMemberRemovedData
}

func (a AuditEventMemberRemoved) GetData() any {
	return a.Data
}

type AuditEventMemberSetRole struct {
	BaseTeamAuditEvent
	Data model.AuditEventMemberSetRoleData
}

func (a AuditEventMemberSetRole) GetData() any {
	return a.Data
}

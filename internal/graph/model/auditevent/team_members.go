package auditevent

import (
	"github.com/nais/api/internal/graph/model"
)

type AuditEventMemberAdded struct {
	BaseAuditEvent
	Data model.AuditEventMemberAddedData
}

func (a AuditEventMemberAdded) GetData() any {
	return a.Data
}

type AuditEventMemberRemoved struct {
	BaseAuditEvent
	Data model.AuditEventMemberRemovedData
}

func (a AuditEventMemberRemoved) GetData() any {
	return a.Data
}

type AuditEventMemberSetRole struct {
	BaseAuditEvent
	Data model.AuditEventMemberSetRoleData
}

func (a AuditEventMemberSetRole) GetData() any {
	return a.Data
}

package auditevent

import (
	"github.com/nais/api/internal/graph/model"
)

type AuditEventMemberAdded struct {
	BaseAuditEvent
	AuditEventMemberAddedData
}

type AuditEventMemberAddedData struct {
	MemberEmail string         `json:"memberEmail"`
	Role        model.TeamRole `json:"role"`
}

func (a AuditEventMemberAdded) GetData() any {
	return a.AuditEventMemberAddedData
}

func NewAuditEventMemberAdded(base BaseAuditEvent, data AuditEventMemberAddedData) AuditEventMemberAdded {
	return AuditEventMemberAdded{
		BaseAuditEvent:            base,
		AuditEventMemberAddedData: data,
	}
}

type AuditEventMemberRemoved struct {
	BaseAuditEvent
	AuditEventMemberRemovedData
}

type AuditEventMemberRemovedData struct {
	MemberEmail string `json:"memberEmail"`
}

func (a AuditEventMemberRemoved) GetData() any {
	return a.AuditEventMemberRemovedData
}

func NewAuditEventMemberRemoved(base BaseAuditEvent, data AuditEventMemberRemovedData) AuditEventMemberRemoved {
	return AuditEventMemberRemoved{
		BaseAuditEvent:              base,
		AuditEventMemberRemovedData: data,
	}
}

type AuditEventMemberSetRole struct {
	BaseAuditEvent
	AuditEventMemberSetRoleData
}

type AuditEventMemberSetRoleData struct {
	MemberEmail string         `json:"memberEmail"`
	Role        model.TeamRole `json:"role"`
}

func (a AuditEventMemberSetRole) GetData() any {
	return a.AuditEventMemberSetRoleData
}

func NewAuditEventMemberSetRole(base BaseAuditEvent, data AuditEventMemberSetRoleData) AuditEventMemberSetRole {
	return AuditEventMemberSetRole{
		BaseAuditEvent:              base,
		AuditEventMemberSetRoleData: data,
	}
}
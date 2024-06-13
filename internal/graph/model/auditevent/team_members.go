package auditevent

import (
	"fmt"

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

func (a AuditEventMemberAdded) IsAuditEvent() {}

func NewAuditEventMemberAdded(base BaseAuditEvent, data AuditEventMemberAddedData) AuditEventMemberAdded {
	return AuditEventMemberAdded{
		BaseAuditEvent:            base.WithMessage(fmt.Sprintf("Added %q with role %q", data.MemberEmail, data.Role)),
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

func (a AuditEventMemberRemoved) IsAuditEvent() {}

func NewAuditEventMemberRemoved(base BaseAuditEvent, data AuditEventMemberRemovedData) AuditEventMemberRemoved {
	return AuditEventMemberRemoved{
		BaseAuditEvent:              base.WithMessage(fmt.Sprintf("Removed %q", data.MemberEmail)),
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

func (a AuditEventMemberSetRole) IsAuditEvent() {}

func NewAuditEventMemberSetRole(base BaseAuditEvent, data AuditEventMemberSetRoleData) AuditEventMemberSetRole {
	return AuditEventMemberSetRole{
		BaseAuditEvent:              base.WithMessage(fmt.Sprintf("Set %q to %q", data.MemberEmail, data.Role)),
		AuditEventMemberSetRoleData: data,
	}
}

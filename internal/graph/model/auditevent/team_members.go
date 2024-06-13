package auditevent

import (
	"fmt"

	"github.com/nais/api/internal/graph/model"
)

type AuditEventAddMember struct {
	BaseAuditEvent
	AuditEventAddMemberData
}

type AuditEventAddMemberData struct {
	MemberEmail string         `json:"memberEmail"`
	Role        model.TeamRole `json:"role"`
}

func (a AuditEventAddMember) GetData() any {
	return a.AuditEventAddMemberData
}

func (a AuditEventAddMember) IsAuditEvent() {}

func NewAuditEventAddMember(base BaseAuditEvent, data AuditEventAddMemberData) AuditEventAddMember {
	return AuditEventAddMember{
		BaseAuditEvent:          base.WithMessage(fmt.Sprintf("Added %q with role %q", data.MemberEmail, data.Role)),
		AuditEventAddMemberData: data,
	}
}

type AuditEventRemoveMember struct {
	BaseAuditEvent
	AuditEventRemoveMemberData
}

type AuditEventRemoveMemberData struct {
	MemberEmail string `json:"memberEmail"`
}

func (a AuditEventRemoveMember) GetData() any {
	return a.AuditEventRemoveMemberData
}

func (a AuditEventRemoveMember) IsAuditEvent() {}

func NewAuditEventRemoveMember(base BaseAuditEvent, data AuditEventRemoveMemberData) AuditEventRemoveMember {
	return AuditEventRemoveMember{
		BaseAuditEvent:             base.WithMessage(fmt.Sprintf("Removed %q", data.MemberEmail)),
		AuditEventRemoveMemberData: data,
	}
}

type AuditEventSetMemberRole struct {
	BaseAuditEvent
	AuditEventSetMemberRoleData
}

type AuditEventSetMemberRoleData struct {
	MemberEmail string         `json:"memberEmail"`
	Role        model.TeamRole `json:"role"`
}

func (a AuditEventSetMemberRole) GetData() any {
	return a.AuditEventSetMemberRoleData
}

func (a AuditEventSetMemberRole) IsAuditEvent() {}

func NewAuditEventSetMemberRole(base BaseAuditEvent, data AuditEventSetMemberRoleData) AuditEventSetMemberRole {
	return AuditEventSetMemberRole{
		BaseAuditEvent:              base.WithMessage(fmt.Sprintf("Set %q to %q", data.MemberEmail, data.Role)),
		AuditEventSetMemberRoleData: data,
	}
}

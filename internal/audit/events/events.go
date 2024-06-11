package audit

import (
	"fmt"
	"github.com/nais/api/internal/database"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
)

// Event represents an audit event:
// Actor performed Action at CreatedAt on ResourceName with ResourceType owned by Team.
// The event may contain additional data.
type Event interface {
	Action() string
	Actor() string
	CreatedAt() time.Time
	Data() any
	ID() uuid.UUID
	Message() string
	ResourceType() string
	ResourceName() string
	Team() slug.Slug
}

// base implements common parts of Event for all events.
type base struct {
	actor     string
	createdAt time.Time
	id        uuid.UUID
	team      slug.Slug
}

func (b base) Actor() string {
	return b.actor
}

func (b base) CreatedAt() time.Time {
	return b.createdAt
}

func (b base) ID() uuid.UUID {
	return b.id
}

func (b base) Team() slug.Slug {
	return b.team
}

type Action string

const (
	ActionTeamAddMember     Action = "ADD_MEMBER"
	ActionTeamRemoveMember  Action = "REMOVE_MEMBER"
	ActionTeamSetMemberRole Action = "SET_MEMBER_ROLE"
	ActionTeamSync          Action = "SYNCHRONIZE_TEAM"
)

type (
	resourceActionMappers map[model.AuditEventResourceType]map[Action]rowMapper
	rowMapper             func(row *database.AuditEvent) (Event, error)
)

var mappers = resourceActionMappers{
	model.AuditEventResourceTypeTeam: {
		ActionTeamSync: teamSyncFromRow,
	},
	model.AuditEventResourceTypeTeamMembers: {
		ActionTeamAddMember:     teamAddMemberFromRow,
		ActionTeamRemoveMember:  teamRemoveMemberFromRow,
		ActionTeamSetMemberRole: teamSetMemberRoleFromRow,
	},
}

func ToEvent(row *database.AuditEvent) (Event, error) {
	resource, ok := mappers[model.AuditEventResourceType(row.ResourceType)]
	if !ok {
		return nil, fmt.Errorf("unsupported resource type %q", row.ResourceType)
	}

	action, ok := resource[Action(row.Action)]
	if !ok {
		return nil, fmt.Errorf("unsupported action %q for resource %q", row.Action, row.ResourceType)
	}

	return action(row)
}

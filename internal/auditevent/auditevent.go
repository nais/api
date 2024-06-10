package auditevent

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
	"time"
)

// Event represents an audit event:
// Actor performed Action at CreatedAt on ResourceName with ResourceType owned by Team.
// The event may contain additional data.
// TODO - should we store correlation_id as well?
type Event interface {
	Action() string
	Actor() string
	CreatedAt() time.Time
	ID() uuid.UUID
	MarshalData() ([]byte, error)
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

type Resource string

const (
	ResourceTeam Resource = "team"
)

type Action string

const (
	ActionTeamAddMember    Action = "add_member"
	ActionTeamRemoveMember Action = "remove_member"
)

type resourceActionMappers map[Resource]map[Action]rowMapper
type rowMapper func(row *gensql.AuditEvent) (Event, error)

var mappers = resourceActionMappers{
	ResourceTeam: {
		ActionTeamAddMember:    teamAddMemberFromRow,
		ActionTeamRemoveMember: teamRemoveMemberFromRow,
	},
}

func ToEvent(row *gensql.AuditEvent) (Event, error) {
	resource, ok := mappers[Resource(row.ResourceType)]
	if !ok {
		return nil, fmt.Errorf("unsupported resource type %q", row.ResourceType)
	}

	action, ok := resource[Action(row.Action)]
	if !ok {
		return nil, fmt.Errorf("unsupported action %q for resource %q", row.Action, row.ResourceType)
	}

	return action(row)
}

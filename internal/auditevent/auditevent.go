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

type EventData interface {
	Marshal() ([]byte, error)
	Valid() bool
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

type RowMapper func(row *gensql.AuditEvent) (Event, error)

var ResourceActions = map[Resource]map[Action]RowMapper{
	ResourceTeam: {
		ActionTeamAddMember:    teamAddMemberFromRow,
		ActionTeamRemoveMember: teamRemoveMemberFromRow,
	},
}

func MapDbRow(row *gensql.AuditEvent) (Event, error) {
	resource, ok := ResourceActions[Resource(row.ResourceType)]
	if !ok {
		return nil, fmt.Errorf("unsupported resource type %q", row.ResourceType)
	}

	action, ok := resource[Action(row.Action)]
	if !ok {
		return nil, fmt.Errorf("unsupported action %q for resource %q", row.Action, row.ResourceType)
	}

	return action(row)
}

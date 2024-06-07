package auditevent

import (
	"fmt"
	"github.com/google/uuid"

	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
	"time"
)

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

type Action string

func DbRowToAuditEvent(row *gensql.AuditEvent) (Event, error) {
	switch row.ResourceType {
	case "team":
		switch row.Action {
		case string(ActionTeamAddMember):
			return teamAddMemberFromRow(row)
		case string(ActionTeamRemoveMember):
			return teamRemoveMemberFromRow(row)
		case "set_member_role":
			panic("TODO")
		}
	}

	return nil, fmt.Errorf("unsupported audit event")
}

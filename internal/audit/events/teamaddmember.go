package audit

import (
	"encoding/json"
	"fmt"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/slug"
)

type teamAddMember struct {
	base
	data teamAddMemberData
}

func (t teamAddMember) Action() string {
	return string(ActionTeamAddMember)
}

func (t teamAddMember) Data() any {
	return t.data
}

func (t teamAddMember) Message() string {
	return fmt.Sprintf("Added %q as %q", t.data.MemberEmail, t.data.Role)
}

func (t teamAddMember) ResourceName() string {
	return t.team.String()
}

func (t teamAddMember) ResourceType() string {
	return string(model.AuditEventResourceTypeTeamMembers)
}

type teamAddMemberData struct {
	Role        string `json:"role"`
	MemberEmail string `json:"member"`
}

// NewTeamAddMember creates an Event for adding a member to a team.
func NewTeamAddMember(actor authz.AuthenticatedUser, teamSlug slug.Slug, memberEmail, role string) Event {
	return &teamAddMember{
		base{
			actor: actor.Identity(),
			team:  teamSlug,
		},
		teamAddMemberData{
			MemberEmail: memberEmail,
			Role:        role,
		},
	}
}

// teamAddMemberFromRow converts a database row to an Event.
func teamAddMemberFromRow(row *database.AuditEvent) (Event, error) {
	var data teamAddMemberData
	if row.Data != nil {
		if err := json.Unmarshal(row.Data, &data); err != nil {
			return nil, err
		}
	}

	return &teamAddMember{
		base{
			actor:     row.Actor,
			createdAt: row.CreatedAt.Time,
			id:        row.ID,
			team:      *row.TeamSlug,
		},
		data,
	}, nil
}

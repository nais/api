package audit

import (
	"encoding/json"
	"fmt"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/slug"
)

// Set member role starts here
type teamSetMemberRole struct {
	base
	data teamSetMemberRoleData
}

type teamSetMemberRoleData struct {
	MemberEmail string `json:"member"`
	Role        string `json:"role"`
}

func (t teamSetMemberRole) Action() string {
	return string(ActionTeamSetMemberRole)
}

func (t teamSetMemberRole) Data() any {
	return t.data
}

func (t teamSetMemberRole) Message() string {
	return fmt.Sprintf("Changed role for %q to %q", t.data.MemberEmail, t.data.Role)
}

func (t teamSetMemberRole) ResourceName() string {
	return t.team.String()
}

func (t teamSetMemberRole) ResourceType() string {
	return string(model.AuditEventResourceTypeTeamMembers)
}

// NewTeamSetMemberRole creates an Event when a member's role is set.
func NewTeamSetMemberRole(actor authz.AuthenticatedUser, teamSlug slug.Slug, memberEmail, role string) Event {
	return &teamSetMemberRole{
		base{
			actor: actor.Identity(),
			team:  teamSlug,
		},
		teamSetMemberRoleData{
			MemberEmail: memberEmail,
			Role:        role,
		},
	}
}

// teamSetMemberRoleFromRow converts a database row to an Event.
func teamSetMemberRoleFromRow(row *database.AuditEvent) (Event, error) {
	var data teamSetMemberRoleData
	if row.Data != nil {
		if err := json.Unmarshal(row.Data, &data); err != nil {
			return nil, err
		}
	}

	return &teamSetMemberRole{
		base{
			actor:     row.Actor,
			createdAt: row.CreatedAt.Time,
			id:        row.ID,
			team:      *row.TeamSlug,
		},
		data,
	}, nil
}

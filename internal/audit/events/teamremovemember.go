package audit

import (
	"encoding/json"
	"fmt"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

type teamRemoveMember struct {
	base
	data teamRemoveMemberData
}

func (t teamRemoveMember) ResourceType() string {
	return string(ResourceTeam)
}

func (t teamRemoveMember) ResourceName() string {
	return t.team.String()
}

func (t teamRemoveMember) Action() string {
	return string(ActionTeamRemoveMember)
}

func (t teamRemoveMember) MarshalData() ([]byte, error) {
	return json.Marshal(t.data)
}

func (t teamRemoveMember) Message() string {
	return fmt.Sprintf("%s removed %s from team", t.actor, t.data.MemberEmail)
}

type teamRemoveMemberData struct {
	MemberEmail string `json:"member"`
}

// NewTeamRemoveMember creates an Event for removing a member from a team.
func NewTeamRemoveMember(actor authz.AuthenticatedUser, teamSlug slug.Slug, memberEmail string) Event {
	return &teamRemoveMember{
		base{
			actor: actor.Identity(),
			team:  teamSlug,
		},
		teamRemoveMemberData{
			MemberEmail: memberEmail,
		},
	}
}

// teamRemoveMemberFromRow converts a database row to an Event.
func teamRemoveMemberFromRow(row *gensql.AuditEvent) (Event, error) {
	var data teamRemoveMemberData
	if row.Data != nil {
		if err := json.Unmarshal(row.Data, &data); err != nil {
			return nil, err
		}
	}

	return &teamRemoveMember{
		base{
			actor:     row.Actor,
			createdAt: row.CreatedAt.Time,
			id:        row.ID,
			team:      *row.TeamSlug,
		},
		data,
	}, nil
}

package auditevent

import (
	"encoding/json"
	"fmt"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

type team struct {
	base
}

func (t team) ResourceType() string {
	return string(ResourceTeam)
}

func (t team) ResourceName() string {
	return t.base.team.String()
}

type teamAddMember struct {
	team
	data teamAddMemberData
}

func (t teamAddMember) Action() string {
	return string(ActionTeamAddMember)
}

func (t teamAddMember) MarshalData() ([]byte, error) {
	return json.Marshal(t.data)
}

func (t teamAddMember) Message() string {
	return fmt.Sprintf("%s added %s as %s to team", t.actor, t.data.MemberEmail, t.data.Role)
}

type teamAddMemberData struct {
	Role        string `json:"role"`
	MemberEmail string `json:"member"`
}

// NewTeamAddMember creates an Event for adding a member to a team.
func NewTeamAddMember(actor authz.AuthenticatedUser, teamSlug slug.Slug, memberEmail, role string) Event {
	return &teamAddMember{
		data: teamAddMemberData{
			MemberEmail: memberEmail,
			Role:        role,
		},
		team: team{
			base{
				actor: actor.Identity(),
				team:  teamSlug,
			},
		},
	}
}

// teamAddMemberFromRow converts a database row to an Event.
func teamAddMemberFromRow(row *gensql.AuditEvent) (Event, error) {
	var data teamAddMemberData
	if row.Data != nil {
		if err := json.Unmarshal(row.Data, &data); err != nil {
			return nil, err
		}
	}

	return &teamAddMember{
		data: data,
		team: team{
			base{
				actor:     row.Actor,
				createdAt: row.CreatedAt.Time,
				id:        row.ID,
				team:      *row.TeamSlug,
			},
		},
	}, nil
}

type teamRemoveMember struct {
	team
	data teamRemoveMemberData
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
		data: teamRemoveMemberData{
			MemberEmail: memberEmail,
		},
		team: team{
			base{
				actor: actor.Identity(),
				team:  teamSlug,
			},
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
		data: data,
		team: team{
			base{
				actor:     row.Actor,
				createdAt: row.CreatedAt.Time,
				id:        row.ID,
				team:      *row.TeamSlug,
			},
		},
	}, nil
}

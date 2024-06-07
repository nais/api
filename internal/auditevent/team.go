package auditevent

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
	"time"
)

const (
	ActionTeamAddMember    Action = "add_member"
	ActionTeamRemoveMember Action = "remove_member"
)

// TODO - look into embedding common fields/member functions for reuse

// TeamAddMember creates an Event for adding a member to a team.
func TeamAddMember(actor authz.AuthenticatedUser, team slug.Slug, memberEmail, role string) Event {
	return &teamAddMember{
		actor: actor.Identity(),
		data: teamAddMemberData{
			MemberEmail: memberEmail,
			Role:        role,
		},
		team: team,
	}
}

// teamAddMemberFromRow converts a database row to an Event.
func teamAddMemberFromRow(row *gensql.AuditEvent) (*teamAddMember, error) {
	var data teamAddMemberData
	if row.Data != nil {
		if err := json.Unmarshal(row.Data, &data); err != nil {
			return nil, err
		}
	}

	return &teamAddMember{
		actor:     row.Actor,
		createdAt: row.CreatedAt.Time,
		data:      data,
		id:        row.ID,
		team:      *row.TeamSlug,
	}, nil
}

type teamAddMember struct {
	actor     string
	createdAt time.Time
	data      teamAddMemberData
	id        uuid.UUID
	team      slug.Slug
}

func (t teamAddMember) Action() string {
	return string(ActionTeamAddMember)
}

func (t teamAddMember) Actor() string {
	return t.actor
}

func (t teamAddMember) CreatedAt() time.Time {
	return t.createdAt
}

func (t teamAddMember) ID() uuid.UUID {
	return t.id
}

func (t teamAddMember) MarshalData() ([]byte, error) {
	return t.data.Marshal()
}

func (t teamAddMember) ResourceType() string {
	return "team"
}

func (t teamAddMember) ResourceName() string {
	return t.team.String()
}

func (t teamAddMember) Team() slug.Slug {
	return t.team
}

func (t teamAddMember) Message() string {
	return fmt.Sprintf("%s added %s as %s to team", t.actor, t.data.MemberEmail, t.data.Role)
}

type teamAddMemberData struct {
	Role        string `json:"role"`
	MemberEmail string `json:"member"`
}

func (f teamAddMemberData) Marshal() ([]byte, error) {
	return json.Marshal(f)
}

func (f teamAddMemberData) Valid() bool {
	return f.Role != "" && f.MemberEmail != ""
}

// TeamRemoveMember creates an Event for removing a member from a team.
func TeamRemoveMember(actor authz.AuthenticatedUser, team slug.Slug, memberEmail string) Event {
	return &teamRemoveMember{
		actor: actor.Identity(),
		data: teamRemoveMemberData{
			MemberEmail: memberEmail,
		},
		team: team,
	}
}

// teamRemoveMemberFromRow converts a database row to an Event.
func teamRemoveMemberFromRow(row *gensql.AuditEvent) (*teamRemoveMember, error) {
	var data teamRemoveMemberData
	if row.Data != nil {
		if err := json.Unmarshal(row.Data, &data); err != nil {
			return nil, err
		}
	}

	return &teamRemoveMember{
		actor:     row.Actor,
		createdAt: row.CreatedAt.Time,
		data:      data,
		id:        row.ID,
		team:      *row.TeamSlug,
	}, nil
}

type teamRemoveMember struct {
	actor     string
	createdAt time.Time
	data      teamRemoveMemberData
	id        uuid.UUID
	team      slug.Slug
}

func (t teamRemoveMember) Action() string {
	return string(ActionTeamRemoveMember)
}

func (t teamRemoveMember) Actor() string {
	return t.actor
}

func (t teamRemoveMember) CreatedAt() time.Time {
	return t.createdAt
}

func (t teamRemoveMember) ID() uuid.UUID {
	return uuid.New()
}

func (t teamRemoveMember) MarshalData() ([]byte, error) {
	return t.data.Marshal()
}

func (t teamRemoveMember) ResourceType() string {
	return "team"
}

func (t teamRemoveMember) ResourceName() string {
	return t.team.String()
}

func (t teamRemoveMember) Team() slug.Slug {
	return t.team
}

func (t teamRemoveMember) Message() string {
	return fmt.Sprintf("%s removed %s from team", t.actor, t.data.MemberEmail)
}

type teamRemoveMemberData struct {
	MemberEmail string `json:"member"`
}

func (f teamRemoveMemberData) Marshal() ([]byte, error) {
	return json.Marshal(f)
}

func (f teamRemoveMemberData) Valid() bool {
	return f.MemberEmail != ""
}

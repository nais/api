package audit

import (
	"fmt"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

type teamSync struct {
	base
}

func (t teamSync) ResourceType() string {
	return string(ResourceTeam)
}

func (t teamSync) ResourceName() string {
	return t.team.String()
}

func (t teamSync) Action() string {
	return string(ActionTeamSync)
}

func (t teamSync) MarshalData() ([]byte, error) {
	return nil, nil
}

func (t teamSync) Message() string {
	return fmt.Sprintf("%s synchronized team", t.actor)
}

type teamSyncData struct {
	Role        string `json:"role"`
	MemberEmail string `json:"member"`
}

// NewTeamSync creates an Event for adding a member to a team.
func NewTeamSync(actor authz.AuthenticatedUser, teamSlug slug.Slug) Event {
	return &teamSync{
		base{
			actor: actor.Identity(),
			team:  teamSlug,
		},
	}
}

// teamSyncFromRow converts a database row to an Event.
func teamSyncFromRow(row *gensql.AuditEvent) (Event, error) {
	return &teamSync{
		base{
			actor:     row.Actor,
			createdAt: row.CreatedAt.Time,
			id:        row.ID,
			team:      *row.TeamSlug,
		},
	}, nil
}

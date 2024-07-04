package team

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/team/teamsql"

	"github.com/nais/api/internal/slug"

	"github.com/nais/api/internal/graphv1/modelv1"
	"github.com/nais/api/internal/graphv1/pagination"
)

type (
	TeamConnection = pagination.Connection[*Team]
	TeamEdge       = pagination.Edge[*Team]
)

type Team struct {
	Slug                   slug.Slug  `json:"slug"`
	Purpose                string     `json:"purpose"`
	AzureGroupID           *uuid.UUID `json:"azureGroupID,omitempty"`
	GitHubTeamSlug         *string    `json:"gitHubTeamSlug,omitempty"`
	GoogleGroupEmail       *string    `json:"googleGroupEmail,omitempty"`
	GoogleArtifactRegistry *string    `json:"googleArtifactRegistry,omitempty"`
	CdnBucket              *string    `json:"cdnBucket,omitempty"`
	LastSuccessfulSync     *time.Time `json:"lastSuccessfulSync,omitempty"`
	DeleteKeyConfirmedAt   *time.Time `json:"-"`
	SlackChannel           string     `json:"slackChannel"`
}

func (t Team) ID() string {
	return t.Slug.String()
}

type TeamOrder struct {
	Field     TeamOrderField         `json:"field"`
	Direction modelv1.OrderDirection `json:"direction"`
}

func (o *TeamOrder) String() string {
	if o == nil {
		return ""
	}

	return strings.ToLower(o.Field.String() + ":" + o.Direction.String())
}

type TeamOrderField string

const (
	TeamOrderFieldSlug TeamOrderField = "SLUG"
)

func (e TeamOrderField) IsValid() bool {
	switch e {
	case TeamOrderFieldSlug:
		return true
	}
	return false
}

func (e TeamOrderField) String() string {
	return string(e)
}

func (e *TeamOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = TeamOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid TeamOrderField", str)
	}
	return nil
}

func (e TeamOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

func toGraphTeam(m *teamsql.Team) *Team {
	ret := &Team{
		Slug:                   m.Slug,
		Purpose:                m.Purpose,
		CdnBucket:              m.CdnBucket,
		SlackChannel:           m.SlackChannel,
		GitHubTeamSlug:         m.GithubTeamSlug,
		AzureGroupID:           m.AzureGroupID,
		GoogleGroupEmail:       m.GoogleGroupEmail,
		GoogleArtifactRegistry: m.GarRepository,
	}

	if m.LastSuccessfulSync.Valid {
		ret.LastSuccessfulSync = &m.LastSuccessfulSync.Time
	}

	if m.DeleteKeyConfirmedAt.Valid {
		ret.DeleteKeyConfirmedAt = &m.DeleteKeyConfirmedAt.Time
	}

	return ret
}

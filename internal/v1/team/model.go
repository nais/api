package team

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/nais/api/internal/v1/graphv1/ident"

	"github.com/google/uuid"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/team/teamsql"
)

type (
	TeamConnection       = pagination.Connection[*Team]
	TeamEdge             = pagination.Edge[*Team]
	TeamMemberConnection = pagination.Connection[*TeamMember]
	TeamMemberEdge       = pagination.Edge[*TeamMember]
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
	SlackChannel           string     `json:"slackChannel"`
	DeleteKeyConfirmedAt   *time.Time `json:"-"`
}

func (Team) IsNode() {}

func (t Team) DeletionInProgress() bool {
	return t.DeleteKeyConfirmedAt != nil
}

func (t Team) ID() ident.Ident {
	return newTeamIdent(t.Slug)
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

func toGraphTeamMember(m *teamsql.ListMembersRow) *TeamMember {
	return &TeamMember{
		Role:     teamRoleFromSqlTeamRole(m.UserRole.RoleName),
		TeamSlug: *m.UserRole.TargetTeamSlug,
		UserID:   m.User.ID,
	}
}

func toGraphUserTeam(m *teamsql.ListForUserRow) *TeamMember {
	return &TeamMember{
		Role:     teamRoleFromSqlTeamRole(m.UserRole.RoleName),
		TeamSlug: *m.UserRole.TargetTeamSlug,
		UserID:   m.User.ID,
	}
}

type TeamMember struct {
	Role     TeamRole
	TeamSlug slug.Slug `json:"-"`
	UserID   uuid.UUID `json:"-"`
}

type TeamRole string

const (
	TeamRoleMember TeamRole = "MEMBER"
	TeamRoleOwner  TeamRole = "OWNER"
)

func (e TeamRole) IsValid() bool {
	switch e {
	case TeamRoleMember, TeamRoleOwner:
		return true
	}
	return false
}

func teamRoleFromSqlTeamRole(t teamsql.RoleName) TeamRole {
	if t == teamsql.RoleNameTeamowner {
		return TeamRoleOwner
	}
	return TeamRoleMember
}

func (e TeamRole) String() string {
	return string(e)
}

func (e *TeamRole) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = TeamRole(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid TeamRole", str)
	}
	return nil
}

func (e TeamRole) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type TeamMemberOrder struct {
	Field     TeamMemberOrderField   `json:"field"`
	Direction modelv1.OrderDirection `json:"direction"`
}

func (o *TeamMemberOrder) String() string {
	if o == nil {
		return ""
	}

	return strings.ToLower(o.Field.String() + ":" + o.Direction.String())
}

type TeamMemberOrderField string

const (
	TeamMemberOrderFieldName  TeamMemberOrderField = "NAME"
	TeamMemberOrderFieldEmail TeamMemberOrderField = "EMAIL"
	TeamMemberOrderFieldRole  TeamMemberOrderField = "ROLE"
)

func (e TeamMemberOrderField) IsValid() bool {
	switch e {
	case TeamMemberOrderFieldName, TeamMemberOrderFieldEmail, TeamMemberOrderFieldRole:
		return true
	}
	return false
}

func (e TeamMemberOrderField) String() string {
	return string(e)
}

func (e *TeamMemberOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = TeamMemberOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid TeamMemberOrderField", str)
	}
	return nil
}

func (e TeamMemberOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type TeamEnvironment struct {
	Name               string    `json:"name"`
	TeamSlug           slug.Slug `json:"-"`
	GCPProjectID       *string   `json:"gcpProjectID,omitempty"`
	SlackAlertsChannel string    `json:"slackAlertsChannel"`
}

func (TeamEnvironment) IsNode() {}

func (e TeamEnvironment) ID() ident.Ident {
	return newTeamEnvironmentIdent(e.TeamSlug, e.Name)
}

func toGraphTeamEnvironment(m *teamsql.TeamAllEnvironment) *TeamEnvironment {
	return &TeamEnvironment{
		Name:               m.Environment,
		TeamSlug:           m.TeamSlug,
		GCPProjectID:       m.GcpProjectID,
		SlackAlertsChannel: m.SlackAlertsChannel,
	}
}

type TeamMembershipOrder struct {
	Field     TeamMembershipOrderField `json:"field"`
	Direction modelv1.OrderDirection   `json:"direction"`
}

func (o *TeamMembershipOrder) String() string {
	if o == nil {
		return ""
	}

	return strings.ToLower(o.Field.String() + ":" + o.Direction.String())
}

type TeamMembershipOrderField string

const (
	TeamMembershipOrderFieldTeamSlug TeamMembershipOrderField = "TEAM_SLUG"
)

func (e TeamMembershipOrderField) IsValid() bool {
	switch e {
	case TeamMembershipOrderFieldTeamSlug:
		return true
	}
	return false
}

func (e TeamMembershipOrderField) String() string {
	return string(e)
}

func (e *TeamMembershipOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = TeamMembershipOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid TeamMembershipOrderField", str)
	}
	return nil
}

func (e TeamMembershipOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

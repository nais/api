package team

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team/teamsql"
	"github.com/nais/api/internal/validate"
	"k8s.io/utils/ptr"
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
	AzureGroupID           *string    `json:"azureGroupID"`
	GitHubTeamSlug         *string    `json:"gitHubTeamSlug"`
	GoogleGroupEmail       *string    `json:"googleGroupEmail"`
	GoogleArtifactRegistry *string    `json:"googleArtifactRegistry"`
	CdnBucket              *string    `json:"cdnBucket"`
	LastSuccessfulSync     *time.Time `json:"lastSuccessfulSync"`
	SlackChannel           string     `json:"slackChannel"`
	DeleteKeyConfirmedAt   *time.Time `json:"-"`
}

type ExternalReferences struct {
	GoogleGroupEmail *string
	AzureGroupID     *uuid.UUID
	GithubTeamSlug   *string
	GarRepository    *string
	CdnBucket        *string
}

func (Team) IsNode()       {}
func (Team) IsSearchNode() {}

func (t Team) DeletionInProgress() bool {
	return t.DeleteKeyConfirmedAt != nil
}

func (t Team) ID() ident.Ident {
	return newTeamIdent(t.Slug)
}

type TeamOrder struct {
	Field     TeamOrderField       `json:"field"`
	Direction model.OrderDirection `json:"direction"`
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
		GoogleGroupEmail:       m.GoogleGroupEmail,
		GoogleArtifactRegistry: m.GarRepository,
	}

	if m.AzureGroupID != nil {
		azureGroupID := m.AzureGroupID.String()
		ret.AzureGroupID = &azureGroupID
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
		Role:     teamMemberRoleFromSqlTeamRole(m.UserRole.RoleName),
		TeamSlug: *m.UserRole.TargetTeamSlug,
		UserID:   m.User.ID,
	}
}

func toGraphUserTeam(m *teamsql.ListForUserRow) *TeamMember {
	return &TeamMember{
		Role:     teamMemberRoleFromSqlTeamRole(m.UserRole.RoleName),
		TeamSlug: *m.UserRole.TargetTeamSlug,
		UserID:   m.User.ID,
	}
}

type TeamMember struct {
	Role     TeamMemberRole
	TeamSlug slug.Slug `json:"-"`
	UserID   uuid.UUID `json:"-"`
}

type TeamMemberRole string

const (
	TeamMemberRoleMember TeamMemberRole = "MEMBER"
	TeamMemberRoleOwner  TeamMemberRole = "OWNER"
)

func (e TeamMemberRole) IsValid() bool {
	switch e {
	case TeamMemberRoleMember, TeamMemberRoleOwner:
		return true
	}
	return false
}

func teamMemberRoleToSqlRole(role TeamMemberRole) teamsql.RoleName {
	if role == TeamMemberRoleMember {
		return teamsql.RoleNameTeammember
	}

	return teamsql.RoleNameTeamowner
}

func teamMemberRoleFromSqlTeamRole(t teamsql.RoleName) TeamMemberRole {
	if t == teamsql.RoleNameTeamowner {
		return TeamMemberRoleOwner
	}
	return TeamMemberRoleMember
}

func (e TeamMemberRole) String() string {
	return string(e)
}

func (e *TeamMemberRole) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = TeamMemberRole(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid TeamMemberRole", str)
	}
	return nil
}

func (e TeamMemberRole) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type TeamMemberOrder struct {
	Field     TeamMemberOrderField `json:"field"`
	Direction model.OrderDirection `json:"direction"`
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
	GCPProjectID       *string   `json:"gcpProjectID"`
	SlackAlertsChannel string    `json:"slackAlertsChannel"`
	TeamSlug           slug.Slug `json:"-"`
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

func toGraphTeamDeleteKey(key *teamsql.TeamDeleteKey) *TeamDeleteKey {
	var confirmedAt *time.Time
	if key.ConfirmedAt.Valid {
		confirmedAt = &key.ConfirmedAt.Time
	}
	return &TeamDeleteKey{
		KeyUUID:         key.Key,
		CreatedAt:       key.CreatedAt.Time,
		ConfirmedAt:     confirmedAt,
		CreatedByUserID: key.CreatedBy,
		TeamSlug:        key.TeamSlug,
	}
}

type UserTeamOrder struct {
	Field     UserTeamOrderField   `json:"field"`
	Direction model.OrderDirection `json:"direction"`
}

func (o *UserTeamOrder) String() string {
	if o == nil {
		return ""
	}

	return strings.ToLower(o.Field.String() + ":" + o.Direction.String())
}

type UserTeamOrderField string

const (
	UserTeamOrderFieldTeamSlug UserTeamOrderField = "TEAM_SLUG"
)

func (e UserTeamOrderField) IsValid() bool {
	switch e {
	case UserTeamOrderFieldTeamSlug:
		return true
	}
	return false
}

func (e UserTeamOrderField) String() string {
	return string(e)
}

func (e *UserTeamOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = UserTeamOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid UserTeamOrderField", str)
	}
	return nil
}

func (e UserTeamOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type CreateTeamInput struct {
	Slug         slug.Slug `json:"slug"`
	Purpose      string    `json:"purpose"`
	SlackChannel string    `json:"slackChannel"`
}

// Rules can be found here: https://api.slack.com/methods/conversations.create#naming
var slackChannelNamePattern = regexp.MustCompile("^#[a-z0-9æøå_-]{2,80}$")

func (i *CreateTeamInput) Validate(ctx context.Context) error {
	verr := validate.New()
	i.Purpose = strings.TrimSpace(i.Purpose)
	i.SlackChannel = strings.TrimSpace(i.SlackChannel)

	if available, err := db(ctx).SlugAvailable(ctx, i.Slug); err != nil {
		return err
	} else if !available {
		verr.Add("slug", "Team slug is not available.")
	}

	if i.Purpose == "" {
		verr.Add("purpose", "This is not a valid purpose.")
	}

	if !slackChannelNamePattern.MatchString(i.SlackChannel) {
		verr.Add("slackChannel", "The Slack channel does not fit the requirements. The name must contain at least 2 characters and at most 80 characters. The name must consist of lowercase letters, numbers, hyphens and underscores, and it must be prefixed with a hash symbol.")
	}

	return verr.NilIfEmpty()
}

type UpdateTeamInput struct {
	Slug         slug.Slug `json:"slug"`
	Purpose      *string   `json:"purpose" `
	SlackChannel *string   `json:"slackChannel"`
}

func (i *UpdateTeamInput) Validate() error {
	verr := validate.New()

	if i.Purpose != nil {
		i.Purpose = ptr.To(strings.TrimSpace(*i.Purpose))
	}

	if i.SlackChannel != nil {
		i.SlackChannel = ptr.To(strings.TrimSpace(*i.SlackChannel))
	}

	if i.Purpose != nil && *i.Purpose == "" {
		verr.Add("purpose", "This is not a valid purpose.")
	}

	if i.SlackChannel != nil {
		if !slackChannelNamePattern.MatchString(*i.SlackChannel) {
			verr.Add("slackChannel", "The Slack channel does not fit the requirements. The name must contain at least 2 characters and at most 80 characters. The name must consist of lowercase letters, numbers, hyphens and underscores, and it must be prefixed with a hash symbol.")
		}
	}

	return verr.NilIfEmpty()
}

type SynchronizeTeamInput struct {
	Slug slug.Slug `json:"slug"`
}

type CreateTeamPayload struct {
	Team *Team `json:"team"`
}

type UpdateTeamPayload struct {
	Team *Team `json:"team"`
}

type SynchronizeTeamPayload struct {
	Team *Team `json:"team"`
}

type RequestTeamDeletionInput struct {
	Slug slug.Slug `json:"slug"`
}

type RequestTeamDeletionPayload struct {
	Key *TeamDeleteKey `json:"key"`
}

type TeamDeleteKey struct {
	KeyUUID         uuid.UUID  `json:"key"`
	CreatedAt       time.Time  `json:"createdAt"`
	ConfirmedAt     *time.Time `json:"-"`
	CreatedByUserID uuid.UUID  `json:"-"`
	TeamSlug        slug.Slug  `json:"-"`
}

func (t TeamDeleteKey) Key() string {
	return t.KeyUUID.String()
}

func (t *TeamDeleteKey) Expires() time.Time {
	return t.CreatedAt.Add(time.Hour)
}

func (t *TeamDeleteKey) HasExpired() bool {
	return time.Now().After(t.Expires())
}

type ConfirmTeamDeletionInput struct {
	Key  string    `json:"key"`
	Slug slug.Slug `json:"slug"`
}

type ConfirmTeamDeletionPayload struct {
	DeletionStarted bool `json:"deletionStarted"`
}

type AddTeamMemberInput struct {
	TeamSlug  slug.Slug      `json:"teamSlug"`
	UserEmail string         `json:"userEmail"`
	Role      TeamMemberRole `json:"role"`
	UserID    uuid.UUID      `json:"-"`
}

type AddTeamMemberPayload struct {
	Member *TeamMember `json:"member"`
}

type RemoveTeamMemberInput struct {
	TeamSlug  slug.Slug `json:"teamSlug"`
	UserEmail string    `json:"userEmail"`
	UserID    uuid.UUID `json:"-"`
}

type RemoveTeamMemberPayload struct {
	UserID   uuid.UUID `json:"-"`
	TeamSlug slug.Slug `json:"-"`
}

type SetTeamMemberRoleInput struct {
	TeamSlug  slug.Slug      `json:"teamSlug"`
	UserEmail string         `json:"userEmail"`
	Role      TeamMemberRole `json:"role"`
	UserID    uuid.UUID      `json:"-"`
}

type SetTeamMemberRolePayload struct {
	Member *TeamMember `json:"member"`
}

type UpdateTeamEnvironmentInput struct {
	Slug               slug.Slug `json:"slug"`
	EnvironmentName    string    `json:"environmentName"`
	SlackAlertsChannel *string   `json:"slackAlertsChannel"`
	GCPProjectID       *string   `json:"-"` // TODO(chredvar): Only used from GRPC, move to separate input / package?
}

func (i *UpdateTeamEnvironmentInput) Validate() error {
	verr := validate.New()

	if i.SlackAlertsChannel != nil {
		s := strings.TrimSpace(*i.SlackAlertsChannel)
		i.SlackAlertsChannel = ptr.To(s)
		if s != "" && (!strings.HasPrefix(s, "#") || len(s) < 3 || len(s) > 80) {
			verr.Add("slackAlertsChannel", "This is not a valid Slack channel name. A valid channel name starts with a '#' and is between 3 and 80 characters long.")
		}
	}

	return verr.NilIfEmpty()
}

type UpdateTeamEnvironmentPayload struct {
	Environment *TeamEnvironment `json:"environment"`
}

type TeamInventoryCounts struct {
	TeamSlug slug.Slug `json:"-"`
}

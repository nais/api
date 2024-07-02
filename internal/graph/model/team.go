package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/slug"
)

type Team struct {
	Slug                   slug.Slug  `json:"slug"`
	Purpose                string     `json:"purpose"`
	LastSuccessfulSync     *time.Time `json:"lastSuccessfulSync,omitempty"`
	SlackChannel           string     `json:"slackChannel"`
	GoogleGroupEmail       *string    `json:"googleGroupEmail"`
	GitHubTeamSlug         *string    `json:"gitHubTeamSlug"`
	AzureGroupID           *uuid.UUID `json:"azureGroupID"`
	GoogleArtifactRegistry *string    `json:"googleArtifactRegistry"`
	CdnBucket              *string    `json:"cdnBucket"`
	DeleteKeyConfirmedAt   *time.Time `json:"deleteKeyConfirmedAt"`
}

func (Team) IsSearchNode() {}

type TeamMemberReconciler struct {
	Enabled bool                        `json:"enabled"`
	GQLVars TeamMemberReconcilerGQLVars `json:"-"`
}

type TeamMemberReconcilerGQLVars struct {
	Name string
}

type TeamSync struct {
	CorrelationID uuid.UUID `json:"correlationID"`
}

type Env struct {
	Team   string                    `json:"-"`
	Name   string                    `json:"name"`
	DBType *database.TeamEnvironment `json:"-"`
}

type TeamDeleteKey struct {
	Key       string               `json:"key"`
	CreatedAt time.Time            `json:"createdAt"`
	Expires   time.Time            `json:"expires"`
	GQLVars   TeamDeleteKeyGQLVars `json:"-"`
}

type TeamDeleteKeyGQLVars struct {
	TeamSlug slug.Slug
	UserID   uuid.UUID
}

type TeamInventory struct {
	SQLInstances []*SQLInstance `json:"sqlInstances"`
}

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

// Team member input.
type TeamMemberInput struct {
	// The ID of user.
	UserID uuid.UUID `json:"userId"`
	// The role that the user will receive.
	Role TeamRole `json:"role"`
	// Reconcilers to opt the team member out of.
	ReconcilerOptOuts []string `json:"reconcilerOptOuts,omitempty"`
}

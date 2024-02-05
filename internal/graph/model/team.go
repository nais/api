package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/slug"
)

// Team type.
type Team struct {
	// Unique slug of the team.
	Slug slug.Slug `json:"slug"`
	// Purpose of the team.
	Purpose string `json:"purpose"`
	// Timestamp of the last successful synchronization of the team.
	LastSuccessfulSync *time.Time `json:"lastSuccessfulSync,omitempty"`
	// Slack channel for the team.
	SlackChannel     string     `json:"slackChannel"`
	GoogleGroupEmail *string    `json:"googleGroupEmail"`
	GitHubTeamSlug   *string    `json:"gitHubTeamSlug"`
	AzureGroupID     *uuid.UUID `json:"azureGroupID"`
}

func (Team) IsSearchNode() {}

// TeamMemberReconciler member reconcilers.
type TeamMemberReconciler struct {
	// Whether or not the reconciler is enabled for the team member.
	Enabled bool `json:"enabled"`

	GQLVars TeamMemberReconcilerGQLVars `json:"-"`
}

type TeamMemberReconcilerGQLVars struct {
	Name string
}

type TeamSync struct {
	CorrelationID uuid.UUID `json:"correlationID"`
}

type Env struct {
	Team string `json:"-"`
	Name string `json:"name"`

	DBType *database.TeamEnvironment `json:"-"`
}

package model

import (
	"time"

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
	SlackChannel string `json:"slackChannel"`
}

func (Team) IsSearchNode() {}

package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/usersync"
)

// User sync run type.
type UserSyncRun struct {
	// The correlation ID of the sync run.
	CorrelationID uuid.UUID `json:"correlationID"`
	// Timestamp of when the run started.
	StartedAt time.Time `json:"startedAt"`
	// Timestamp of when the run finished.
	FinishedAt *time.Time `json:"finishedAt,omitempty"`

	GQLVars UserSyncRunGQLVars `json:"-"`
}

type UserSyncRunGQLVars struct {
	Status usersync.RunStatus
	Error  error
}

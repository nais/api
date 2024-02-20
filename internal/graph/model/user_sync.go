package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/usersync"
)

type UserSyncRun struct {
	CorrelationID uuid.UUID          `json:"correlationID"`
	StartedAt     time.Time          `json:"startedAt"`
	FinishedAt    *time.Time         `json:"finishedAt,omitempty"`
	GQLVars       UserSyncRunGQLVars `json:"-"`
}

type UserSyncRunGQLVars struct {
	Status usersync.RunStatus
	Error  error
}

package model

import (
	"time"

	"github.com/google/uuid"

	"github.com/nais/api/internal/graph/scalar"
)

type UsersyncRun struct {
	ID         scalar.Ident `json:"id"`
	StartedAt  time.Time    `json:"startedAt"`
	FinishedAt time.Time    `json:"finishedAt"`
	Error      *string      `json:"error"`
	GQLVars    UsersyncRunGQLVars
}

type UsersyncRunGQLVars struct {
	CorrelationID uuid.UUID
}

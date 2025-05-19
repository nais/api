package servicemaintenance

import (
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
)

type (
	ValkeyMaintenanceUpdateConnection = pagination.Connection[*ValkeyMaintenanceUpdate]
	ValkeyMaintenanceUpdateEdge       = pagination.Edge[*ValkeyMaintenanceUpdate]
)

type ValkeyMaintenanceUpdate struct {
	*AivenUpdate
}

func (ValkeyMaintenanceUpdate) IsServiceMaintenanceUpdate() {}

type StartValkeyMaintenanceInput struct {
	ServiceName     string    `json:"serviceName"`
	TeamSlug        slug.Slug `json:"teamSlug"`
	EnvironmentName string    `json:"environmentName"`
}

type StartValkeyMaintenancePayload struct {
	Error *string `json:"error,omitempty"`
}

type ValkeyMaintenance struct {
	AivenProject string `json:"-"`
	ServiceName  string `json:"-"`
}

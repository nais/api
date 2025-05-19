package servicemaintenance

import (
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
)

type (
	OpenSearchMaintenanceUpdateConnection = pagination.Connection[*OpenSearchMaintenanceUpdate]
	OpenSearchMaintenanceUpdateEdge       = pagination.Edge[*OpenSearchMaintenanceUpdate]
)

type OpenSearchMaintenanceUpdate struct {
	*AivenUpdate
}

func (OpenSearchMaintenanceUpdate) IsServiceMaintenanceUpdate() {}

type StartOpenSearchMaintenanceInput struct {
	ServiceName     string    `json:"serviceName"`
	TeamSlug        slug.Slug `json:"teamSlug"`
	EnvironmentName string    `json:"environmentName"`
}

type StartOpenSearchMaintenancePayload struct {
	Error *string `json:"error,omitempty"`
}

type OpenSearchMaintenance struct {
	Updates *OpenSearchMaintenanceUpdateConnection `json:"updates"`
}

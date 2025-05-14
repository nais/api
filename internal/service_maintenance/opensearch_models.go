package servicemaintenance

import (
	"time"

	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
)

type (
	OpenSearchMaintenanceUpdateConnection = pagination.Connection[*OpenSearchMaintenanceUpdate]
	OpenSearchMaintenanceUpdateEdge       = pagination.Edge[*OpenSearchMaintenanceUpdate]
)

type OpenSearchMaintenanceUpdate struct {
	// Title of the maintenance.
	Title string `json:"title"`
	// Description of the maintenance.
	Description string `json:"description"`
	// Deadline for installing the maintenance. If set, maintenance is mandatory and will be forcibly applied.
	Deadline *time.Time `json:"deadline,omitempty"`
	// The time when the update will be automatically applied. If set, maintenance is mandatory and will be forcibly applied.
	StartAt *time.Time `json:"startAt,omitempty"`
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

package servicemaintenance

import (
	"time"

	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
)

type (
	ValkeyMaintenanceUpdateConnection = pagination.Connection[*ValkeyMaintenanceUpdate]
	ValkeyMaintenanceUpdateEdge       = pagination.Edge[*ValkeyMaintenanceUpdate]
)

type ValkeyMaintenanceUpdate struct {
	// Title of the maintenance.
	Title string `json:"title"`
	// Description of the maintenance.
	Description string `json:"description"`
	// Deadline for installing the maintenance. If set, maintenance is mandatory and will be forcibly applied.
	Deadline *time.Time `json:"deadline,omitempty"`
	// The time when the update will be automatically applied. If set, maintenance is mandatory and will be forcibly applied.
	StartAt *time.Time `json:"startAt,omitempty"`
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
	Updates *ValkeyMaintenanceUpdateConnection `json:"updates"`
}

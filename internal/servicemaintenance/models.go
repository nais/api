package servicemaintenance

import (
	"time"

	aiven_service "github.com/aiven/go-client-codegen/handler/service"
	"github.com/nais/api/internal/graph/pagination"
)

type ServiceMaintenanceUpdate interface {
	IsServiceMaintenanceUpdate()
}

type AivenAPIMaintenance struct {
	Updates []aiven_service.UpdateOut
}

type AivenMaintenance[T any] struct {
	Updates *pagination.Connection[*AivenUpdate]
}

type AivenUpdate struct {
	// Title of the maintenance.
	Title string `json:"title"`
	// Description of the maintenance.
	Description string `json:"description"`
	// Deadline for installing the maintenance. If set, maintenance is mandatory and will be forcibly applied.
	Deadline *time.Time `json:"deadline,omitempty"`
	// The time when the update will be automatically applied. If set, maintenance is mandatory and will be forcibly applied.
	StartAt *time.Time `json:"startAt,omitempty"`
}

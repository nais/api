package servicemaintenance

import (
	"time"

	aiven_service "github.com/aiven/go-client-codegen/handler/service"
)

type ServiceMaintenanceUpdate interface {
	IsServiceMaintenanceUpdate()
}

type AivenAPIMaintenance struct {
	Updates []aiven_service.UpdateOut
}

type AivenMaintenance[T any, E any] struct {
	Updates []AivenUpdate[E]
}

type AivenUpdate[T any] struct {
	// Title of the maintenance.
	Title string `json:"title"`
	// Description of the maintenance.
	Description string `json:"description"`
	// Deadline for installing the maintenance. If set, maintenance is mandatory and will be forcibly applied.
	Deadline *time.Time `json:"deadline,omitempty"`
	// The time when the update will be automatically applied. If set, maintenance is mandatory and will be forcibly applied.
	StartAt *time.Time `json:"startAt,omitempty"`
}

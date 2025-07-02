package servicemaintenance

import (
	"time"

	"github.com/nais/api/internal/graph/model"
)

type ServiceMaintenanceUpdate interface {
	IsServiceMaintenanceUpdate()
}

type MaintenanceWindow struct {
	// Day of the week when the maintenance is scheduled.
	DayOfWeek model.Weekday `json:"dayOfWeek"`
	// Time of day when the maintenance is scheduled.
	TimeOfDay string `json:"timeOfDay"`
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

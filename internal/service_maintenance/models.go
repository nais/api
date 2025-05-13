package servicemaintenance

import (
	"time"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
)

type RunMaintenancePayload struct {
	Error *string `json:"error,omitempty"`
}

type ServiceMaintenanceUpdate struct {
	Deadline          *time.Time `json:"deadline,omitempty"`
	Title             string     `json:"title"`
	Description       string     `json:"description"`
	DocumentationLink *string    `json:"documentation_link,omitempty"`
	StartAt           *time.Time `json:"start_at,omitempty"`
}

type ServiceMaintenance struct {
	Identifier ident.Ident                `json:"id"`
	Updates    []ServiceMaintenanceUpdate `json:"updates"`
}

type RunMaintenanceInput struct {
	EnvironmentName string      `json:"environmentName"`
	Project         string      `json:"project"`
	ServiceName     string      `json:"serviceName"`
	TeamSlug        slug.Slug   `json:"teamSlug"`
	ServiceType     ServiceType `json:"serviceType"`
}

func (ServiceMaintenance) IsNode() {}
func (i *ServiceMaintenance) ID() ident.Ident {
	return i.Identifier
}

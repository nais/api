package servicemaintenance

import (
	"time"

	"github.com/nais/api/internal/graph/ident"
)

type ServiceMaintenanceUpdate struct {
	Deadline          *time.Time `json:"deadline,omitempty"`
	Title             string     `json:"title"`
	Description       string     `json:"description"`
	DocumentationLink *string    `json:"documentation_link,omitempty"`
	StartAfter        *time.Time `json:"start_after,omitempty"`
	StartAt           *time.Time `json:"start_at,omitempty"`
}

type ServiceMaintenance struct {
	Identifier ident.Ident                `json:"id"`
	Updates    []ServiceMaintenanceUpdate `json:"updates"`
}

func (ServiceMaintenance) IsNode() {}
func (i *ServiceMaintenance) ID() ident.Ident {
	return i.Identifier
}

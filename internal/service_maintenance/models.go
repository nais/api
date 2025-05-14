package servicemaintenance

import (
	aiven_service "github.com/aiven/go-client-codegen/handler/service"
)

type ServiceMaintenanceUpdate interface {
	IsServiceMaintenanceUpdate()
}

type AivenMaintenance struct {
	Updates []aiven_service.UpdateOut
}

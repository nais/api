package aivencache

import (
	"context"

	"github.com/aiven/go-client-codegen/handler/service"
)

type AivenClient interface {
	ServiceGet(context.Context, string, string, ...[2]string) (*service.ServiceGetOut, error)
	ServiceMaintenanceStart(context.Context, string, string) error
}

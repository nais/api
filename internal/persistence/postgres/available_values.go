package postgres

import (
	"context"

	"github.com/nais/api/internal/environment"
	"github.com/nais/api/internal/kubernetes/crd"
)

const (
	majorVersionsEnumName = "postgresMajorVersions"
	extensionsEnumName    = "postgresExtensions"
)

func init() {
	crd.RegisterEnum(crd.Enum{
		Name:    majorVersionsEnumName,
		CRDName: "postgres.nais.io",
		Version: "v1",
		Path:    []string{"spec", "majorVersion"},
	})
	crd.RegisterEnum(crd.Enum{
		Name:    extensionsEnumName,
		CRDName: "postgres.nais.io",
		Version: "v1",
		Path:    []string{"spec", "extensions", "items", "name"},
	})
}

func AvailableMajorVersions(ctx context.Context, environmentName string) (*environment.AvailableValueOptions, error) {
	return environment.GetAvailableValueOptions(ctx, environmentName, majorVersionsEnumName)
}

func AvailableExtensions(ctx context.Context, environmentName string) (*environment.AvailableValueOptions, error) {
	return environment.GetAvailableValueOptions(ctx, environmentName, extensionsEnumName)
}

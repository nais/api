package postgres

import (
	"context"

	"github.com/nais/api/internal/environment"
	"github.com/nais/api/internal/kubernetes/crd"
)

const majorVersionsEnumName = "postgresMajorVersions"

func init() {
	crd.RegisterEnum(crd.Enum{
		Name:    majorVersionsEnumName,
		CRDName: "postgres.nais.io",
		Version: "v1",
		Path:    []string{"spec", "majorVersion"},
	})
}

func AvailableMajorVersions(ctx context.Context, environmentName string) (*environment.AvailableValueOptions, error) {
	return environment.GetAvailableValueOptions(ctx, environmentName, majorVersionsEnumName)
}

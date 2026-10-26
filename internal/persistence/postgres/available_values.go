package postgres

import (
	"context"

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

func AvailableMajorVersions(ctx context.Context, environmentName string) ([]string, error) {
	return crd.Values(ctx, environmentName, majorVersionsEnumName)
}

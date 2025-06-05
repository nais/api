package status

import (
	"context"

	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
)

type checkDeprecatedPostgresVersion struct{}

func (c checkDeprecatedPostgresVersion) Run(ctx context.Context, w workload.Workload) ([]WorkloadStatusError, WorkloadState) {
	errors := make([]WorkloadStatusError, 0)

	var sqlInstances []nais_io_v1.CloudSqlInstance

	switch v := w.(type) {
	case *application.Application:
		if v.Spec != nil && v.Spec.GCP != nil && len(v.Spec.GCP.SqlInstances) > 0 {
			sqlInstances = v.Spec.GCP.SqlInstances
		}
	case *job.Job:
		if v.Spec != nil && v.Spec.GCP != nil && len(v.Spec.GCP.SqlInstances) > 0 {
			sqlInstances = v.Spec.GCP.SqlInstances
		}
	default:
		return errors, WorkloadStateNais
	}

	for _, instance := range sqlInstances {
		if err := checkPostgresVersion(instance.Type); err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		return errors, WorkloadStateNotNais
	}

	return errors, WorkloadStateNais
}

func checkPostgresVersion(instanceType nais_io_v1.CloudSqlInstanceType) WorkloadStatusError {
	switch instanceType {
	case nais_io_v1.CloudSqlInstanceTypePostgres12:
		return WorkloadStatusUnsupportedCloudSQLVersion{
			Level:   WorkloadStatusErrorLevelError,
			Version: string(instanceType),
		}
	case nais_io_v1.CloudSqlInstanceTypePostgres13:
		return WorkloadStatusUnsupportedCloudSQLVersion{
			Level:   WorkloadStatusErrorLevelWarning,
			Version: string(instanceType),
		}
	default:
		return nil
	}
}

func (checkDeprecatedPostgresVersion) Supports(w workload.Workload) bool {
	return true
}

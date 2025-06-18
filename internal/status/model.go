package status

import (
	"fmt"
	"io"
	"strconv"

	"github.com/nais/api/internal/vulnerability"
)

type WorkloadStatusError interface {
	GetLevel() WorkloadStatusErrorLevel
	GetName() string
}

type WorkloadStatusDeprecatedIngress struct {
	Level   WorkloadStatusErrorLevel `json:"level"`
	Ingress string                   `json:"ingress"`
}

func (w WorkloadStatusDeprecatedIngress) GetLevel() WorkloadStatusErrorLevel { return w.Level }

func (w WorkloadStatusDeprecatedIngress) GetName() string {
	return WorkloadStatusErrorTypeDeprecatedIngress.String()
}

type WorkloadStatusDeprecatedRegistry struct {
	Level      WorkloadStatusErrorLevel `json:"level"`
	Registry   string                   `json:"registry"`
	Repository string                   `json:"repository"`
	Name       string                   `json:"name"`
	Tag        string                   `json:"tag"`
}

func (w WorkloadStatusDeprecatedRegistry) GetLevel() WorkloadStatusErrorLevel { return w.Level }

func (w WorkloadStatusDeprecatedRegistry) GetName() string {
	return WorkloadStatusErrorTypeDeprecatedRegistry.String()
}

type WorkloadStatusInvalidNaisYaml struct {
	Level  WorkloadStatusErrorLevel `json:"level"`
	Detail string                   `json:"detail"`
}

func (w WorkloadStatusInvalidNaisYaml) GetLevel() WorkloadStatusErrorLevel { return w.Level }

func (w WorkloadStatusInvalidNaisYaml) GetName() string {
	return WorkloadStatusErrorTypeInvalidNaisYaml.String()
}

type WorkloadStatusNoRunningInstances struct {
	Level WorkloadStatusErrorLevel `json:"level"`
}

func (w WorkloadStatusNoRunningInstances) GetLevel() WorkloadStatusErrorLevel { return w.Level }

func (w WorkloadStatusNoRunningInstances) GetName() string {
	return WorkloadStatusErrorTypeNoRunningInstances.String()
}

type WorkloadStatusSynchronizationFailing struct {
	Level  WorkloadStatusErrorLevel `json:"level"`
	Detail string                   `json:"detail"`
}

func (w WorkloadStatusSynchronizationFailing) GetLevel() WorkloadStatusErrorLevel { return w.Level }

func (w WorkloadStatusSynchronizationFailing) GetName() string {
	return WorkloadStatusErrorTypeSynchronizationFailing.String()
}

type WorkloadStatusFailedRun struct {
	Level  WorkloadStatusErrorLevel `json:"level"`
	Detail string                   `json:"message"`
	Name   string                   `json:"name"`
}

func (w WorkloadStatusFailedRun) GetLevel() WorkloadStatusErrorLevel { return w.Level }

func (w WorkloadStatusFailedRun) GetName() string {
	return WorkloadStatusErrorTypeFailedRun.String()
}

type WorkloadStatusMissingSBOM struct {
	Level WorkloadStatusErrorLevel `json:"level"`
}

func (w WorkloadStatusMissingSBOM) GetLevel() WorkloadStatusErrorLevel { return w.Level }

func (w WorkloadStatusMissingSBOM) GetName() string {
	return WorkloadStatusErrorTypeMissingSbom.String()
}

type WorkloadStatusVulnerable struct {
	Level   WorkloadStatusErrorLevel                 `json:"level"`
	Summary *vulnerability.ImageVulnerabilitySummary `json:"summary"`
}

func (w WorkloadStatusVulnerable) GetLevel() WorkloadStatusErrorLevel { return w.Level }

func (w WorkloadStatusVulnerable) GetName() string {
	return WorkloadStatusErrorTypeVulnerable.String()
}

type WorkloadStatusUnsupportedCloudSQLVersion struct {
	Level   WorkloadStatusErrorLevel `json:"level"`
	Version string                   `json:"version"`
}

func (w WorkloadStatusUnsupportedCloudSQLVersion) GetLevel() WorkloadStatusErrorLevel { return w.Level }

func (w WorkloadStatusUnsupportedCloudSQLVersion) GetName() string {
	return WorkloadStatusErrorTypeUnsupportedCloudSQLVersion.String()
}

type WorkloadStatus struct {
	State  WorkloadState         `json:"state"`
	Errors []WorkloadStatusError `json:"errors"`
}

type WorkloadState int

const (
	WorkloadStateUnknown WorkloadState = iota
	WorkloadStateNais
	WorkloadStateNotNais
	WorkloadStateFailing
)

func (e WorkloadState) IsValid() bool {
	switch e {
	case WorkloadStateNais, WorkloadStateNotNais, WorkloadStateFailing, WorkloadStateUnknown:
		return true
	}
	return false
}

func (e WorkloadState) String() string {
	switch e {
	case WorkloadStateNais:
		return "NAIS"
	case WorkloadStateNotNais:
		return "NOT_NAIS"
	case WorkloadStateFailing:
		return "FAILING"
	}
	return "UNKNOWN"
}

func (e *WorkloadState) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	switch str {
	case "NAIS":
		*e = WorkloadStateNais
	case "NOT_NAIS":
		*e = WorkloadStateNotNais
	case "FAILING":
		*e = WorkloadStateFailing
	default:
		return fmt.Errorf("%s is not a valid WorkloadState", str)
	}

	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid WorkloadState", str)
	}
	return nil
}

func (e WorkloadState) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type WorkloadStatusErrorLevel int

const (
	WorkloadStatusErrorLevelUnknown WorkloadStatusErrorLevel = iota
	WorkloadStatusErrorLevelTodo
	WorkloadStatusErrorLevelWarning
	WorkloadStatusErrorLevelError
)

var AllWorkloadStatusErrorLevel = []WorkloadStatusErrorLevel{
	WorkloadStatusErrorLevelTodo,
	WorkloadStatusErrorLevelWarning,
	WorkloadStatusErrorLevelError,
}

func (e WorkloadStatusErrorLevel) IsValid() bool {
	switch e {
	case WorkloadStatusErrorLevelTodo, WorkloadStatusErrorLevelWarning, WorkloadStatusErrorLevelError:
		return true
	}
	return false
}

func (e WorkloadStatusErrorLevel) String() string {
	switch e {
	case WorkloadStatusErrorLevelTodo:
		return "TODO"
	case WorkloadStatusErrorLevelWarning:
		return "WARNING"
	case WorkloadStatusErrorLevelError:
		return "ERROR"
	}
	return "UNKNOWN"
}

func (e *WorkloadStatusErrorLevel) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	switch str {
	case "TODO":
		*e = WorkloadStatusErrorLevelTodo
	case "WARNING":
		*e = WorkloadStatusErrorLevelWarning
	case "ERROR":
		*e = WorkloadStatusErrorLevelError
	default:
		return fmt.Errorf("%s is not a valid WorkloadStatusErrorLevel", str)
	}

	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid WorkloadStatusErrorLevel", str)
	}
	return nil
}

func (e WorkloadStatusErrorLevel) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type WorkloadStatusErrorType string

const (
	// Error describing usage of a deprecated registry
	WorkloadStatusErrorTypeDeprecatedRegistry WorkloadStatusErrorType = "DEPRECATED_REGISTRY"
	// Error describing usage of a deprecated ingress
	WorkloadStatusErrorTypeDeprecatedIngress WorkloadStatusErrorType = "DEPRECATED_INGRESS"
	// Error describing usage of an instance of Cloud SQL with a unsupported version
	WorkloadStatusErrorTypeUnsupportedCloudSQLVersion WorkloadStatusErrorType = "UNSUPPORTED_CLOUD_SQL_VERSION"
	// Error describing that the workload is failing to synchronize
	WorkloadStatusErrorTypeSynchronizationFailing WorkloadStatusErrorType = "SYNCHRONIZATION_FAILING"
	// Error describing that the workload is failing to run
	WorkloadStatusErrorTypeFailedRun WorkloadStatusErrorType = "FAILED_RUN"
	// Error describing that the workload is missing SBOM
	WorkloadStatusErrorTypeMissingSbom WorkloadStatusErrorType = "MISSING_SBOM"
	// Error describing that the workload is vulnerable
	WorkloadStatusErrorTypeVulnerable WorkloadStatusErrorType = "VULNERABLE"
	// Workload does not have any active instances, but is requested to have some
	WorkloadStatusErrorTypeNoRunningInstances WorkloadStatusErrorType = "NO_RUNNING_INSTANCES"
	// Last deployed YAML is invalid
	WorkloadStatusErrorTypeInvalidNaisYaml WorkloadStatusErrorType = "INVALID_NAIS_YAML"
)

var AllWorkloadStatusErrorType = []WorkloadStatusErrorType{
	WorkloadStatusErrorTypeDeprecatedRegistry,
	WorkloadStatusErrorTypeDeprecatedIngress,
	WorkloadStatusErrorTypeUnsupportedCloudSQLVersion,
	WorkloadStatusErrorTypeSynchronizationFailing,
	WorkloadStatusErrorTypeFailedRun,
	WorkloadStatusErrorTypeMissingSbom,
	WorkloadStatusErrorTypeVulnerable,
	WorkloadStatusErrorTypeNoRunningInstances,
	WorkloadStatusErrorTypeInvalidNaisYaml,
}

func (e WorkloadStatusErrorType) IsValid() bool {
	switch e {
	case WorkloadStatusErrorTypeDeprecatedRegistry, WorkloadStatusErrorTypeDeprecatedIngress, WorkloadStatusErrorTypeUnsupportedCloudSQLVersion, WorkloadStatusErrorTypeSynchronizationFailing, WorkloadStatusErrorTypeFailedRun, WorkloadStatusErrorTypeMissingSbom, WorkloadStatusErrorTypeVulnerable, WorkloadStatusErrorTypeNoRunningInstances, WorkloadStatusErrorTypeInvalidNaisYaml:
		return true
	}
	return false
}

func (e WorkloadStatusErrorType) String() string {
	return string(e)
}

func (e *WorkloadStatusErrorType) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = WorkloadStatusErrorType(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid WorkloadStatusErrorType", str)
	}
	return nil
}

func (e WorkloadStatusErrorType) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

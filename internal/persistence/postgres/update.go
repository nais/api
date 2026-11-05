package postgres

import (
	"fmt"

	"github.com/nais/api/internal/kubernetes"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// preparePostgresUpdate changes only the fields requested through the public API.
// The typed view is used for validation and audit differences, never as the whole
// update payload: newer operator fields and metadata must survive older API types.
func preparePostgresUpdate(existing *unstructured.Unstructured, input UpdatePostgresInput) (*unstructured.Unstructured, []*PostgresUpdatedActivityLogEntryDataUpdatedField, error) {
	pg, err := kubernetes.ToConcrete[nais_io_v1.Postgres](existing)
	if err != nil {
		return nil, nil, err
	}
	var changes []*PostgresUpdatedActivityLogEntryDataUpdatedField
	for _, update := range []func(*nais_io_v1.Postgres, UpdatePostgresInput) ([]*PostgresUpdatedActivityLogEntryDataUpdatedField, error){
		updatePostgresUserLabels,
		updateHighAvailability,
		updateResources,
		updateExtensions,
	} {
		fields, err := update(pg, input)
		if err != nil {
			return nil, nil, err
		}
		changes = append(changes, fields...)
	}

	result := existing.DeepCopy()
	if len(changes) == 0 {
		return result, nil, nil
	}
	desired, err := toUnstructuredWithoutZeroResources(pg)
	if err != nil {
		return nil, nil, err
	}
	for _, change := range changes {
		var path []string
		switch change.Field {
		case "labels":
			path = []string{"metadata", "labels"}
		case "highAvailability", "extensions":
			path = []string{"spec", change.Field}
		case "cpu", "memory", "diskSize":
			path = []string{"spec", "resources", change.Field}
		default:
			return nil, nil, fmt.Errorf("unsupported Postgres update field %q", change.Field)
		}
		value, found, err := unstructured.NestedFieldCopy(desired.Object, path...)
		if err != nil {
			return nil, nil, err
		}
		if !found {
			unstructured.RemoveNestedField(result.Object, path...)
		} else if err := unstructured.SetNestedField(result.Object, value, path...); err != nil {
			return nil, nil, err
		}
	}
	return result, changes, nil
}

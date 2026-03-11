package apply

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// FieldChange represents a single field that changed between the before and after states.
type FieldChange struct {
	// Field is the dot-separated path to the changed field, e.g. "spec.replicas".
	Field string `json:"field"`

	// OldValue is the value before the apply. Nil if the field was added.
	OldValue any `json:"oldValue,omitempty"`

	// NewValue is the value after the apply. Nil if the field was removed.
	NewValue any `json:"newValue,omitempty"`
}

// ignoredTopLevelFields are fields managed by the Kubernetes API server that
// should be excluded from diffs as they are not user-controlled.
var ignoredTopLevelFields = map[string]bool{
	"status": true,
}

// ignoredMetadataFields are metadata fields managed by the API server.
var ignoredMetadataFields = map[string]bool{
	"resourceVersion":            true,
	"uid":                        true,
	"generation":                 true,
	"creationTimestamp":          true,
	"managedFields":              true,
	"selfLink":                   true,
	"deletionTimestamp":          true,
	"deletionGracePeriodSeconds": true,
}

// Diff compares two unstructured Kubernetes objects and returns a list of field changes.
// If before is nil, all fields in after are considered "added".
// If after is nil, all fields in before are considered "removed".
// Server-managed fields (status, metadata.resourceVersion, etc.) are excluded.
func Diff(before, after *unstructured.Unstructured) []FieldChange {
	var beforeMap, afterMap map[string]any

	if before != nil {
		beforeMap = before.Object
	}
	if after != nil {
		afterMap = after.Object
	}

	changes := diffMaps(beforeMap, afterMap, "")

	// Sort for deterministic output
	slices.SortFunc(changes, func(a, b FieldChange) int {
		return strings.Compare(a.Field, b.Field)
	})

	return changes
}

// diffMaps recursively compares two maps and collects field changes.
func diffMaps(before, after map[string]any, prefix string) []FieldChange {
	var changes []FieldChange

	// Collect all keys from both maps
	allKeys := map[string]struct{}{}
	for k := range before {
		allKeys[k] = struct{}{}
	}
	for k := range after {
		allKeys[k] = struct{}{}
	}

	for key := range allKeys {
		fieldPath := joinPath(prefix, key)

		if shouldIgnoreField(prefix, key) {
			continue
		}

		oldVal, oldExists := before[key]
		newVal, newExists := after[key]

		switch {
		case !oldExists && newExists:
			// Field was added
			changes = append(changes, flattenAdded(fieldPath, newVal)...)
		case oldExists && !newExists:
			// Field was removed
			changes = append(changes, flattenRemoved(fieldPath, oldVal)...)
		case oldExists && newExists:
			// Field exists in both — compare values
			changes = append(changes, diffValues(fieldPath, oldVal, newVal)...)
		}
	}

	return changes
}

// diffValues compares two values at a given path and returns changes.
func diffValues(path string, oldVal, newVal any) []FieldChange {
	// If both are maps, recurse
	oldMap, oldIsMap := toMap(oldVal)
	newMap, newIsMap := toMap(newVal)
	if oldIsMap && newIsMap {
		return diffMaps(oldMap, newMap, path)
	}

	// If both are slices, compare them
	oldSlice, oldIsSlice := toSlice(oldVal)
	newSlice, newIsSlice := toSlice(newVal)
	if oldIsSlice && newIsSlice {
		return diffSlices(path, oldSlice, newSlice)
	}

	// Scalar comparison
	if !reflect.DeepEqual(oldVal, newVal) {
		return []FieldChange{{
			Field:    path,
			OldValue: oldVal,
			NewValue: newVal,
		}}
	}

	return nil
}

// diffSlices compares two slices. If elements are maps, it compares element-by-element.
// Otherwise it compares the slices as a whole.
func diffSlices(path string, oldSlice, newSlice []any) []FieldChange {
	var changes []FieldChange

	maxLen := max(len(oldSlice), len(newSlice))
	for i := range maxLen {
		elemPath := fmt.Sprintf("%s[%d]", path, i)

		switch {
		case i >= len(oldSlice):
			changes = append(changes, flattenAdded(elemPath, newSlice[i])...)
		case i >= len(newSlice):
			changes = append(changes, flattenRemoved(elemPath, oldSlice[i])...)
		default:
			changes = append(changes, diffValues(elemPath, oldSlice[i], newSlice[i])...)
		}
	}

	return changes
}

// flattenAdded returns FieldChanges for a newly added value (possibly nested).
func flattenAdded(path string, val any) []FieldChange {
	if m, ok := toMap(val); ok {
		var changes []FieldChange
		for k, v := range m {
			changes = append(changes, flattenAdded(joinPath(path, k), v)...)
		}
		if len(changes) == 0 {
			// Empty map added
			return []FieldChange{{Field: path, NewValue: val}}
		}
		return changes
	}

	if s, ok := toSlice(val); ok {
		var changes []FieldChange
		for i, v := range s {
			changes = append(changes, flattenAdded(fmt.Sprintf("%s[%d]", path, i), v)...)
		}
		if len(changes) == 0 {
			return []FieldChange{{Field: path, NewValue: val}}
		}
		return changes
	}

	return []FieldChange{{Field: path, NewValue: val}}
}

// flattenRemoved returns FieldChanges for a removed value (possibly nested).
func flattenRemoved(path string, val any) []FieldChange {
	if m, ok := toMap(val); ok {
		var changes []FieldChange
		for k, v := range m {
			changes = append(changes, flattenRemoved(joinPath(path, k), v)...)
		}
		if len(changes) == 0 {
			return []FieldChange{{Field: path, OldValue: val}}
		}
		return changes
	}

	if s, ok := toSlice(val); ok {
		var changes []FieldChange
		for i, v := range s {
			changes = append(changes, flattenRemoved(fmt.Sprintf("%s[%d]", path, i), v)...)
		}
		if len(changes) == 0 {
			return []FieldChange{{Field: path, OldValue: val}}
		}
		return changes
	}

	return []FieldChange{{Field: path, OldValue: val}}
}

// shouldIgnoreField returns true if the field should be excluded from the diff.
func shouldIgnoreField(prefix, key string) bool {
	if prefix == "" && ignoredTopLevelFields[key] {
		return true
	}

	if prefix == "metadata" && ignoredMetadataFields[key] {
		return true
	}

	return false
}

// joinPath joins two path segments with a dot separator.
func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

// toMap attempts to cast a value to map[string]any.
func toMap(val any) (map[string]any, bool) {
	m, ok := val.(map[string]any)
	return m, ok
}

// toSlice attempts to cast a value to []any.
func toSlice(val any) ([]any, bool) {
	s, ok := val.([]any)
	return s, ok
}

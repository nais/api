package model

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/validation"
)

// UserLabelPrefix is the namespace under which user-defined labels are stored on
// Nais resources. Only labels with this prefix are user-editable and exposed
// through the API; all other labels are considered platform-internal.
const UserLabelPrefix = "labels.nais.io/"

type ResourceLabel struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// LabelFilter matches any resource carrying Key when Value is nil, or the exact
// Key/Value pair when Value is set.
type LabelFilter struct {
	Key   string  `json:"key"`
	Value *string `json:"value,omitempty"`
}

func (l *LabelFilter) Requirement() (*labels.Requirement, error) {
	if l.Value == nil {
		return labels.NewRequirement(l.Key, selection.Exists, nil)
	}
	return labels.NewRequirement(l.Key, selection.Equals, []string{*l.Value})
}

type LabelFilters []*LabelFilter

func (l LabelFilters) Selector() labels.Selector {
	selector := labels.NewSelector()
	for _, f := range l {
		req, err := f.Requirement()
		if err != nil {
			continue
		}
		selector = selector.Add(*req)
	}
	return selector
}

type LabelValidationError struct {
	Message string
}

func (e LabelValidationError) Error() string {
	return e.Message
}

func (e LabelValidationError) GraphError() string {
	return e.Message
}

// ValidateUserLabels validates that user labels are well-formed and carry the required prefix.
func ValidateUserLabels(labels []*ResourceLabel) error {
	seen := make(map[string]struct{}, len(labels))
	for _, l := range labels {
		if l == nil {
			continue
		}
		if _, dup := seen[l.Key]; dup {
			return LabelValidationError{Message: fmt.Sprintf("Duplicate label key %q.", l.Key)}
		}
		seen[l.Key] = struct{}{}

		if !strings.HasPrefix(l.Key, UserLabelPrefix) {
			return LabelValidationError{Message: fmt.Sprintf("label key %q must be prefixed with %q", l.Key, UserLabelPrefix)}
		}

		for _, msg := range validation.IsQualifiedName(l.Key) {
			return LabelValidationError{Message: fmt.Sprintf("Invalid label key %q: %s.", l.Key, msg)}
		}
		for _, msg := range validation.IsValidLabelValue(l.Value) {
			return LabelValidationError{Message: fmt.Sprintf("Invalid value for label %q: %s.", l.Key, msg)}
		}
	}
	return nil
}

func (l *LabelFilters) UnmarshalGQL(v any) error {
	inputList, ok := v.([]any)
	if !ok {
		return nil
	}

	var filters LabelFilters
	for _, item := range inputList {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		key, _ := itemMap["key"].(string)
		value, _ := itemMap["value"].(string)

		if !strings.HasPrefix(key, UserLabelPrefix) {
			return LabelValidationError{Message: fmt.Sprintf("label key %q must be prefixed with %q", key, UserLabelPrefix)}
		}

		var valuePtr *string
		if value != "" {
			valuePtr = &value
		}

		filters = append(filters, &LabelFilter{
			Key:   key,
			Value: valuePtr,
		})
	}

	*l = filters
	return nil
}

func (l LabelFilters) MarshalGQL(w io.Writer) {
	var output []map[string]any
	for _, filter := range l {
		item := map[string]any{
			"key": filter.Key,
		}
		if filter.Value != nil {
			item["value"] = *filter.Value
		}
		output = append(output, item)
	}

	_ = json.NewEncoder(w).Encode(output)
}

// UserLabels extracts the user-defined labels from a Kubernetes label map. Only
// labels prefixed with UserLabelPrefix are returned.
// The result is sorted by key for stable output.
func UserLabels(labels map[string]string) []*ResourceLabel {
	out := make([]*ResourceLabel, 0, len(labels))
	for k, v := range labels {
		if !strings.HasPrefix(k, UserLabelPrefix) && k != UserLabelPrefix {
			continue
		}
		out = append(out, &ResourceLabel{Key: k, Value: v})
	}

	slices.SortFunc(out, func(a, b *ResourceLabel) int {
		return strings.Compare(a.Key, b.Key)
	})

	return out
}

// FormatUserLabels joins a slice of ResourceLabel into a string of the form "key1=value1, key2=value2"
func FormatUserLabels(labels []*ResourceLabel) string {
	parts := make([]string, 0, len(labels))
	for _, l := range labels {
		if l == nil {
			continue
		}
		parts = append(parts, l.Key+"="+l.Value)
	}
	return strings.Join(parts, ", ")
}

// MatchesLabelFilters reports whether the given user labels satisfy all of the
// provided filters (AND semantics). The label keys are compared in their
// full form, i.e. including the UserLabelPrefix.
func MatchesLabelFilters(labels []*ResourceLabel, filters []*LabelFilter) bool {
	if len(filters) == 0 {
		return true
	}

	index := make(map[string]string, len(labels))
	for _, l := range labels {
		if l == nil {
			continue
		}
		index[l.Key] = l.Value
	}

	for _, f := range filters {
		if f == nil || f.Key == "" {
			continue
		}
		v, ok := index[f.Key]
		if !ok {
			return false
		}
		if f.Value != nil && v != *f.Value {
			return false
		}
	}

	return true
}

// MergeUserLabels returns a copy of existing where every user-label entry (those
// prefixed with UserLabelPrefix) is replaced by the provided desired labels.
// Platform-internal labels are preserved untouched. Passing an empty desired
// slice therefore removes all user labels.
func MergeUserLabels(existing map[string]string, desired []*ResourceLabel) map[string]string {
	out := make(map[string]string, len(existing)+len(desired))
	for k, v := range existing {
		if strings.HasPrefix(k, UserLabelPrefix) {
			continue
		}
		out[k] = v
	}

	for _, l := range desired {
		if l == nil {
			continue
		}
		out[l.Key] = l.Value
	}

	return out
}

package model

import (
	"encoding/json"
	"io"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
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

func (l *LabelFilters) UnmarshalGQL(v any) error {
	// The GraphQL input is expected to be a list of objects with "key" and optional "value" fields. We need to convert this into our internal LabelFilters type.
	inputList, ok := v.([]any)
	if !ok {
		return nil // or return an error if you prefer
	}

	var filters LabelFilters
	for _, item := range inputList {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue // skip invalid items
		}

		key, _ := itemMap["key"].(string)
		value, _ := itemMap["value"].(string)

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
	// Convert our internal LabelFilters type into a format suitable for GraphQL output, which is a list of objects with "key" and optional "value" fields.
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
// labels prefixed with UserLabelPrefix are returned, with the prefix stripped.
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

// MatchesLabelFilters reports whether the given user labels satisfy all of the
// provided filters (AND semantics). The label keys are compared in their
// stripped form, i.e. without the UserLabelPrefix.
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
		out[UserLabelPrefix+l.Key] = l.Value
	}

	return out
}

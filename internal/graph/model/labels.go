package model

import (
	"sort"
	"strings"
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

// UserLabels extracts the user-defined labels from a Kubernetes label map. Only
// labels prefixed with UserLabelPrefix are returned, with the prefix stripped.
// The result is sorted by key for stable output.
func UserLabels(labels map[string]string) []*ResourceLabel {
	out := make([]*ResourceLabel, 0, len(labels))
	for k, v := range labels {
		key, ok := strings.CutPrefix(k, UserLabelPrefix)
		if !ok || key == "" {
			continue
		}
		out = append(out, &ResourceLabel{Key: key, Value: v})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Key < out[j].Key
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

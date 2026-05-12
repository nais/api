package activitylog

import (
	"slices"
)

type filter struct {
	action       ActivityLogEntryAction
	resourceType []ActivityLogEntryResourceType
}

var knownFilters = map[ActivityLogActivityType]filter{}

// reverseFilters maps "resource_type:action" strings to their ActivityLogActivityType values.
var reverseFilters = map[string][]ActivityLogActivityType{}

func RegisterFilter(activityType ActivityLogActivityType, action ActivityLogEntryAction, resourceType ActivityLogEntryResourceType) {
	if f, ok := knownFilters[activityType]; ok {
		if f.action == action {
			// If the activity type is already registered with the same action, append the resource type
			f.resourceType = append(f.resourceType, resourceType)
			// Make sure the resource type slice is unique
			slices.Sort(f.resourceType)
			f.resourceType = slices.Compact(f.resourceType)

			knownFilters[activityType] = f
			rebuildReverseFilters()
			return
		}
		panic("filter already registered: " + string(activityType) + " with action " + string(f.action))
	}
	knownFilters[activityType] = filter{
		action:       action,
		resourceType: []ActivityLogEntryResourceType{resourceType},
	}
	rebuildReverseFilters()
}

func rebuildReverseFilters() {
	reverseFilters = make(map[string][]ActivityLogActivityType)
	for activityType, f := range knownFilters {
		for _, rt := range f.resourceType {
			key := string(rt) + ":" + string(f.action)
			reverseFilters[key] = append(reverseFilters[key], activityType)
		}
	}
}

// LookupActivityTypes returns all ActivityLogActivityType values that match the given resource_type:action combination.
func LookupActivityTypes(resourceType, action string) []ActivityLogActivityType {
	key := resourceType + ":" + action
	return reverseFilters[key]
}

func withFilters(filter *ActivityLogFilter) []string {
	if filter == nil {
		return nil
	}

	var ret []string
	for _, f := range filter.ActivityTypes {
		kf, ok := knownFilters[f]
		if !ok {
			continue
		}
		for _, resourceType := range kf.resourceType {
			ret = append(ret, string(resourceType)+":"+string(kf.action))
		}
	}

	return ret
}

func withResourceTypes(filter *ActivityLogFilter) []string {
	if filter == nil || len(filter.ResourceTypes) == 0 {
		return nil
	}

	ret := make([]string, len(filter.ResourceTypes))
	for i, rt := range filter.ResourceTypes {
		ret[i] = string(rt)
	}
	return ret
}

func withEnvironments(filter *ActivityLogFilter) []string {
	if filter == nil || len(filter.Environments) == 0 {
		return nil
	}

	return filter.Environments
}

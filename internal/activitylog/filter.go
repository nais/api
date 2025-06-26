package activitylog

import (
	"slices"
)

type filter struct {
	action       ActivityLogEntryAction
	resourceType []ActivityLogEntryResourceType
}

var knownFilters = map[ActivityLogActivityType]filter{}

func RegisterFilter(activityType ActivityLogActivityType, action ActivityLogEntryAction, resourceType ActivityLogEntryResourceType) {
	if f, ok := knownFilters[activityType]; ok {
		if f.action == action {
			// If the activity type is already registered with the same action, append the resource type
			f.resourceType = append(f.resourceType, resourceType)
			// Make sure the resouce type slice is unique
			slices.Sort(f.resourceType)
			f.resourceType = slices.Compact(f.resourceType)

			knownFilters[activityType] = f
			return
		}
		panic("filter already registered: " + string(activityType) + " with action " + string(f.action))
	}
	knownFilters[activityType] = filter{
		action:       action,
		resourceType: []ActivityLogEntryResourceType{resourceType},
	}
}

func withFilters(filter *ActivityLogFilter) [][]string {
	if filter == nil {
		return nil
	}

	var ret [][]string
	for _, f := range filter.ActivityTypes {
		kf, ok := knownFilters[f]
		if !ok {
			continue
		}
		for _, resourceType := range kf.resourceType {
			ret = append(ret, []string{
				string(resourceType),
				string(kf.action),
			})
		}
	}

	return ret
}

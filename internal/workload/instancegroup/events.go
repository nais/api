package instancegroup

import (
	"context"
	"fmt"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var eventsGVR = schema.GroupVersionResource{
	Group:    "events.k8s.io",
	Version:  "v1",
	Resource: "events",
}

// fetchEvents fetches K8s events for the given involved object in the namespace.
func fetchEvents(ctx context.Context, client dynamic.Interface, namespace, involvedKind, involvedName string) ([]unstructured.Unstructured, error) {
	fieldSelector := fmt.Sprintf("regarding.kind=%s,regarding.name=%s", involvedKind, involvedName)

	list, err := client.Resource(eventsGVR).Namespace(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("list events for %s/%s: %w", involvedKind, involvedName, err)
	}

	return list.Items, nil
}

// ListEvents returns events for an instance group and all its pods.
// Events are sorted by timestamp, newest first.
func ListEvents(ctx context.Context, ig *InstanceGroup) ([]*InstanceGroupEvent, error) {
	l := fromContext(ctx)
	client, err := l.k8sClient(ig.EnvironmentName)
	if err != nil {
		return nil, err
	}

	namespace := ig.TeamSlug.String()
	var allEvents []*InstanceGroupEvent

	// Fetch events for the ReplicaSet itself
	rsEvents, err := fetchEvents(ctx, client, namespace, "ReplicaSet", ig.Name)
	if err != nil {
		l.log.WithError(err).WithField("replicaset", ig.Name).Warn("failed to fetch ReplicaSet events")
	} else {
		for i := range rsEvents {
			if ev := toEvent(&rsEvents[i], nil); ev != nil {
				allEvents = append(allEvents, ev)
			}
		}
	}

	// Fetch events for each pod in the instance group
	instances, err := ListInstances(ctx, ig)
	if err != nil {
		l.log.WithError(err).Warn("failed to list instances for event fetching")
	} else {
		for _, inst := range instances {
			podEvents, err := fetchEvents(ctx, client, namespace, "Pod", inst.Name)
			if err != nil {
				l.log.WithError(err).WithField("pod", inst.Name).Warn("failed to fetch pod events")
				continue
			}
			podName := inst.Name
			for i := range podEvents {
				if ev := toEvent(&podEvents[i], &podName); ev != nil {
					allEvents = append(allEvents, ev)
				}
			}
		}
	}

	// Sort by timestamp, newest first
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Timestamp.After(allEvents[j].Timestamp)
	})

	return allEvents, nil
}

// ListInstanceEvents returns events for a single pod/instance.
func ListInstanceEvents(ctx context.Context, ig *InstanceGroup, instanceName string) ([]*InstanceGroupEvent, error) {
	l := fromContext(ctx)
	client, err := l.k8sClient(ig.EnvironmentName)
	if err != nil {
		return nil, err
	}

	podEvents, err := fetchEvents(ctx, client, ig.TeamSlug.String(), "Pod", instanceName)
	if err != nil {
		return nil, fmt.Errorf("fetch events for pod %s: %w", instanceName, err)
	}

	var events []*InstanceGroupEvent
	for i := range podEvents {
		if ev := toEvent(&podEvents[i], &instanceName); ev != nil {
			events = append(events, ev)
		}
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.After(events[j].Timestamp)
	})

	return events, nil
}

// toEvent converts a raw K8s event into an InstanceGroupEvent.
func toEvent(obj *unstructured.Unstructured, sourceInstance *string) *InstanceGroupEvent {
	reason, _, _ := unstructured.NestedString(obj.Object, "reason")
	note, _, _ := unstructured.NestedString(obj.Object, "note")
	eventType, _, _ := unstructured.NestedString(obj.Object, "type")

	timestamp := extractEventTimestamp(obj)
	if timestamp.IsZero() {
		return nil
	}

	severity := InstanceGroupEventSeverityInfo
	if eventType == "Warning" {
		severity = InstanceGroupEventSeverityWarning
	}

	return &InstanceGroupEvent{
		Timestamp:      timestamp,
		Reason:         reason,
		Message:        note,
		Severity:       severity,
		SourceInstance: sourceInstance,
	}
}

// extractEventTimestamp gets the most relevant timestamp from a K8s event.
func extractEventTimestamp(obj *unstructured.Unstructured) time.Time {
	// Try series.lastObservedTime first (for recurring events)
	if ts, ok, _ := unstructured.NestedString(obj.Object, "series", "lastObservedTime"); ok && ts != "" {
		if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			return t
		}
	}

	// Try deprecatedLastTimestamp
	if ts, ok, _ := unstructured.NestedString(obj.Object, "deprecatedLastTimestamp"); ok && ts != "" {
		if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			return t
		}
	}

	// Fall back to metadata.creationTimestamp
	if ts, ok, _ := unstructured.NestedString(obj.Object, "metadata", "creationTimestamp"); ok && ts != "" {
		if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			return t
		}
	}

	return time.Time{}
}

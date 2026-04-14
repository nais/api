package instancegroup

import (
	"context"
	"fmt"
	"sort"
	"strings"
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

// ListEvents returns translated events for an instance group and all its pods.
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
			if ev := translateEvent(&rsEvents[i], nil); ev != nil {
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
				if ev := translateEvent(&podEvents[i], &podName); ev != nil {
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

// ListInstanceEvents returns translated events for a single pod/instance.
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
		if ev := translateEvent(&podEvents[i], &instanceName); ev != nil {
			events = append(events, ev)
		}
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.After(events[j].Timestamp)
	})

	return events, nil
}

// translateEvent converts a raw K8s event into a user-friendly InstanceGroupEvent.
func translateEvent(obj *unstructured.Unstructured, sourceInstance *string) *InstanceGroupEvent {
	reason, _, _ := unstructured.NestedString(obj.Object, "reason")
	note, _, _ := unstructured.NestedString(obj.Object, "note")
	eventType, _, _ := unstructured.NestedString(obj.Object, "type")

	timestamp := extractEventTimestamp(obj)
	if timestamp.IsZero() {
		return nil
	}

	message, severity := translateReasonAndNote(reason, note, eventType)

	return &InstanceGroupEvent{
		Timestamp:      timestamp,
		Message:        message,
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

// translateReasonAndNote maps K8s event reasons and notes to user-friendly messages.
func translateReasonAndNote(reason, note, eventType string) (string, InstanceGroupEventSeverity) {
	noteLower := strings.ToLower(note)

	switch reason {
	// --- Scheduling ---
	case "FailedScheduling":
		return classifySchedulingFailure(note), InstanceGroupEventSeverityError

	case "Scheduled":
		return "Instance has been assigned to a node and will start shortly.", InstanceGroupEventSeverityInfo

	// --- Image ---
	case "Pulling":
		return "Downloading container image...", InstanceGroupEventSeverityInfo

	case "Pulled":
		return "Container image downloaded successfully.", InstanceGroupEventSeverityInfo

	case "Failed":
		if strings.Contains(noteLower, "image") || strings.Contains(noteLower, "pull") || strings.Contains(noteLower, "errimagepull") {
			return "Failed to download container image. Check that the image exists and access is configured correctly.", InstanceGroupEventSeverityError
		}
		if strings.Contains(noteLower, "mount") || strings.Contains(noteLower, "volume") {
			return classifyMountFailure(note), InstanceGroupEventSeverityError
		}
		return fmt.Sprintf("Operation failed: %s", note), InstanceGroupEventSeverityError

	case "InspectFailed":
		return "Failed to inspect container image. The image may be corrupted or inaccessible.", InstanceGroupEventSeverityError

	case "ErrImageNeverPull":
		return "Container image is not available locally and pull policy prevents downloading it.", InstanceGroupEventSeverityError

	// --- Image pull backoff ---
	case "BackOff":
		if strings.Contains(noteLower, "image") || strings.Contains(noteLower, "pull") {
			return "Repeated failures downloading container image. The image may not exist or the registry may be inaccessible.", InstanceGroupEventSeverityError
		}
		return "Instance is crash-looping — it keeps crashing shortly after starting. Check application logs for details.", InstanceGroupEventSeverityError

	// --- Container lifecycle ---
	case "Created":
		return "Container created.", InstanceGroupEventSeverityInfo

	case "Started":
		return "Container started.", InstanceGroupEventSeverityInfo

	case "Killing":
		if strings.Contains(noteLower, "liveness") {
			return "Instance failed health check (liveness probe) and is being restarted.", InstanceGroupEventSeverityWarning
		}
		if strings.Contains(noteLower, "preempt") {
			return "Instance is being shut down to make room for higher-priority workloads.", InstanceGroupEventSeverityWarning
		}
		return "Instance is being terminated.", InstanceGroupEventSeverityInfo

	// --- Probes ---
	case "Unhealthy":
		return classifyProbeFailure(note), InstanceGroupEventSeverityWarning

	// --- OOM ---
	case "OOMKilling", "OOMKilled":
		return "Instance ran out of memory and was terminated. Consider increasing the memory limit in your nais.yaml.", InstanceGroupEventSeverityError

	// --- Eviction ---
	case "Evicted":
		return "Instance was evicted due to resource pressure on the node (e.g. low memory or disk space).", InstanceGroupEventSeverityWarning

	case "Preempting":
		return "Instance is being preempted to make room for higher-priority workloads.", InstanceGroupEventSeverityWarning

	// --- Volume/mount issues ---
	case "FailedMount":
		return classifyMountFailure(note), InstanceGroupEventSeverityWarning

	case "FailedAttachVolume":
		return "Failed to attach storage volume. This is usually a transient infrastructure issue.", InstanceGroupEventSeverityWarning

	// --- Scaling ---
	case "SuccessfulRescale":
		return classifyRescale(note), InstanceGroupEventSeverityInfo

	case "FailedRescale":
		return fmt.Sprintf("Autoscaler failed to adjust instance count: %s", note), InstanceGroupEventSeverityWarning

	// --- ReplicaSet ---
	case "SuccessfulCreate":
		return "New instance created by the instance group.", InstanceGroupEventSeverityInfo

	case "SuccessfulDelete":
		return "Instance removed from the instance group.", InstanceGroupEventSeverityInfo

	case "FailedCreate":
		return fmt.Sprintf("Failed to create a new instance: %s", note), InstanceGroupEventSeverityError

	// --- Network ---
	case "NetworkNotReady":
		return "Network is not ready for this instance. This is usually a transient infrastructure issue.", InstanceGroupEventSeverityWarning

	// --- Node ---
	case "NodeNotReady":
		return "The node running this instance is not ready. The instance may be rescheduled.", InstanceGroupEventSeverityWarning

	case "NodeNotSchedulable":
		return "The node is marked as unschedulable. The instance may need to be moved.", InstanceGroupEventSeverityWarning

	default:
		// Fall through to severity-based default
		severity := InstanceGroupEventSeverityInfo
		if eventType == "Warning" {
			severity = InstanceGroupEventSeverityWarning
		}

		if note != "" {
			return note, severity
		}
		return fmt.Sprintf("Event: %s", reason), severity
	}
}

// classifySchedulingFailure provides a user-friendly message for scheduling failures.
func classifySchedulingFailure(note string) string {
	noteLower := strings.ToLower(note)

	switch {
	case strings.Contains(noteLower, "insufficient memory"):
		return "Unable to start instance: not enough memory available in the cluster. Consider reducing the memory request in your nais.yaml."
	case strings.Contains(noteLower, "insufficient cpu"):
		return "Unable to start instance: not enough CPU available in the cluster. Consider reducing the CPU request in your nais.yaml."
	case strings.Contains(noteLower, "persistentvolumeclaim"):
		return "Unable to start instance: a required storage volume is not available."
	case strings.Contains(noteLower, "taint") || strings.Contains(noteLower, "toleration"):
		return "Unable to start instance: no suitable nodes available due to node restrictions."
	case strings.Contains(noteLower, "affinity"):
		return "Unable to start instance: no nodes match the required placement constraints."
	default:
		return fmt.Sprintf("Unable to schedule instance: %s", note)
	}
}

// classifyProbeFailure provides a user-friendly message for probe failures.
func classifyProbeFailure(note string) string {
	noteLower := strings.ToLower(note)

	switch {
	case strings.Contains(noteLower, "liveness"):
		detail := extractProbeDetail(note)
		if detail != "" {
			return fmt.Sprintf("Instance failed health check (liveness probe) and will be restarted. %s", detail)
		}
		return "Instance failed health check (liveness probe) and will be restarted."

	case strings.Contains(noteLower, "readiness"):
		detail := extractProbeDetail(note)
		if detail != "" {
			return fmt.Sprintf("Instance is not ready to receive traffic (readiness probe failed). %s", detail)
		}
		return "Instance is not ready to receive traffic (readiness probe failed)."

	case strings.Contains(noteLower, "startup"):
		detail := extractProbeDetail(note)
		if detail != "" {
			return fmt.Sprintf("Instance is still starting up (startup probe failed). %s", detail)
		}
		return "Instance is still starting up (startup probe failed). If this persists, the instance may be stuck in a crash loop."

	default:
		return fmt.Sprintf("Health check failed: %s", note)
	}
}

// extractProbeDetail extracts useful detail from a probe failure note.
func extractProbeDetail(note string) string {
	noteLower := strings.ToLower(note)

	switch {
	case strings.Contains(noteLower, "connection refused"):
		return "The application is not accepting connections on the expected port."
	case strings.Contains(noteLower, "statuscode: 5"):
		return "The health endpoint returned a server error."
	case strings.Contains(noteLower, "statuscode: 404"):
		return "The health endpoint was not found (404). Check that the path is configured correctly."
	case strings.Contains(noteLower, "timeout"):
		return "The health check timed out. The application may be overloaded or unresponsive."
	default:
		return ""
	}
}

// classifyMountFailure provides a user-friendly message for mount failures.
func classifyMountFailure(note string) string {
	noteLower := strings.ToLower(note)

	switch {
	case strings.Contains(noteLower, "secret") && strings.Contains(noteLower, "not found"):
		if name := extractResourceName(note, "secret"); name != "" {
			return fmt.Sprintf("Failed to mount volume: secret %q does not exist. It may not have been created yet.", name)
		}
		return "Failed to mount volume: a referenced Secret does not exist. It may not have been created yet."
	case strings.Contains(noteLower, "configmap") && strings.Contains(noteLower, "not found"):
		if name := extractResourceName(note, "configmap"); name != "" {
			return fmt.Sprintf("Failed to mount volume: configmap `%s` does not exist.", name)
		}
		return "Failed to mount volume: a referenced ConfigMap does not exist."
	case strings.Contains(noteLower, "timeout"):
		return "Timed out waiting for volume to be mounted. This is usually a transient issue."
	default:
		return fmt.Sprintf("Failed to mount volume: %s", note)
	}
}

// extractResourceName extracts a quoted resource name following the given keyword from a K8s event note.
// For example, from `secret "my-secret" not found` it extracts `my-secret`.
func extractResourceName(note, keyword string) string {
	noteLower := strings.ToLower(note)
	idx := strings.Index(noteLower, keyword)
	if idx == -1 {
		return ""
	}
	// Find the first quote after the keyword
	rest := note[idx+len(keyword):]
	start := strings.IndexByte(rest, '"')
	if start == -1 {
		return ""
	}
	rest = rest[start+1:]
	end := strings.IndexByte(rest, '"')
	if end == -1 {
		return ""
	}
	return rest[:end]
}

// classifyRescale provides a user-friendly message for autoscale events.
func classifyRescale(note string) string {
	noteLower := strings.ToLower(note)

	switch {
	case strings.Contains(noteLower, "above"):
		return "Autoscaler increased instance count due to high resource usage."
	case strings.Contains(noteLower, "below"):
		return "Autoscaler decreased instance count due to low resource usage."
	default:
		return fmt.Sprintf("Autoscaler adjusted instance count: %s", note)
	}
}

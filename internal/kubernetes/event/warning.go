package event

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nais/api/internal/kubernetes/translate"
)

type WorkloadWarningEvent struct {
	WorkloadEventBase
	Kind        WorkloadWarningEventKind
	Instance    *string
	Count       int
	FirstSeenAt time.Time
}

// warningEventData is the data persisted for warning events. Notes are stored untranslated so translations can be
// improved without losing information.
type warningEventData struct {
	Note      string    `json:"note"`
	Instance  string    `json:"instance,omitempty"`
	Count     int       `json:"count"`
	FirstSeen time.Time `json:"firstSeen"`
}

type WorkloadWarningEventKind string

const (
	WorkloadWarningEventKindPlacementFailed        WorkloadWarningEventKind = "PLACEMENT_FAILED"
	WorkloadWarningEventKindMountFailed            WorkloadWarningEventKind = "MOUNT_FAILED"
	WorkloadWarningEventKindImagePullFailed        WorkloadWarningEventKind = "IMAGE_PULL_FAILED"
	WorkloadWarningEventKindStartFailed            WorkloadWarningEventKind = "START_FAILED"
	WorkloadWarningEventKindCrashLoop              WorkloadWarningEventKind = "CRASH_LOOP"
	WorkloadWarningEventKindHealthCheckFailed      WorkloadWarningEventKind = "HEALTH_CHECK_FAILED"
	WorkloadWarningEventKindInstanceStopped        WorkloadWarningEventKind = "INSTANCE_STOPPED"
	WorkloadWarningEventKindInstanceCreationFailed WorkloadWarningEventKind = "INSTANCE_CREATION_FAILED"
	WorkloadWarningEventKindScalingFailed          WorkloadWarningEventKind = "SCALING_FAILED"
	WorkloadWarningEventKindRunFailed              WorkloadWarningEventKind = "RUN_FAILED"
)

var AllWorkloadWarningEventKind = []WorkloadWarningEventKind{
	WorkloadWarningEventKindPlacementFailed,
	WorkloadWarningEventKindMountFailed,
	WorkloadWarningEventKindImagePullFailed,
	WorkloadWarningEventKindStartFailed,
	WorkloadWarningEventKindCrashLoop,
	WorkloadWarningEventKindHealthCheckFailed,
	WorkloadWarningEventKindInstanceStopped,
	WorkloadWarningEventKindInstanceCreationFailed,
	WorkloadWarningEventKindScalingFailed,
	WorkloadWarningEventKindRunFailed,
}

func (e WorkloadWarningEventKind) IsValid() bool {
	return slices.Contains(AllWorkloadWarningEventKind, e)
}

func (e WorkloadWarningEventKind) String() string {
	return string(e)
}

func (e *WorkloadWarningEventKind) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}
	*e = WorkloadWarningEventKind(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid WorkloadWarningEventKind", str)
	}
	return nil
}

func (e WorkloadWarningEventKind) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

// warningTranslator translates the note of a Kubernetes event into Nais wording. ok is false for notes that should
// not be exposed, for example duplicates of other events.
type warningTranslator func(note string) (kind WorkloadWarningEventKind, message string, ok bool)

var warningTranslators = map[string]warningTranslator{
	"FailedScheduling":             translateFailedScheduling,
	"FailedMount":                  translateFailedMount,
	"FailedAttachVolume":           translateFailedMount,
	"Failed":                       translateFailed,
	"BackOff":                      translateBackOff,
	"Unhealthy":                    translateUnhealthy,
	"Evicted":                      translateEvicted,
	"FailedCreate":                 translateFailedCreate,
	"FailedGetResourceMetric":      translateFailedMetrics,
	"FailedGetPodsMetric":          translateFailedMetrics,
	"FailedGetObjectMetric":        translateFailedMetrics,
	"FailedGetExternalMetric":      translateFailedMetrics,
	"FailedComputeMetricsReplicas": translateFailedMetrics,
	"FailedRescale":                translateFailedRescale,
	"BackoffLimitExceeded":         fixedWarning(WorkloadWarningEventKindRunFailed, "The run failed too many times and will not be retried."),
	"DeadlineExceeded":             fixedWarning(WorkloadWarningEventKindRunFailed, "The run took longer than allowed and was stopped."),
}

// warningReasons are the Kubernetes event reasons exposed as warning events. Repeated occurrences of these are
// aggregated into a single event.
var warningReasons = slices.Sorted(maps.Keys(warningTranslators))

// supportedReasons are all Kubernetes event reasons exposed as workload events.
var supportedReasons = append([]string{"SuccessfulRescale", "Killing"}, warningReasons...)

func translateWarning(reason, note string) (WorkloadWarningEventKind, string, bool) {
	translate, ok := warningTranslators[reason]
	if !ok {
		return "", "", false
	}
	return translate(strings.TrimPrefix(note, "(combined from similar events): "))
}

func fixedWarning(kind WorkloadWarningEventKind, message string) warningTranslator {
	return func(string) (WorkloadWarningEventKind, string, bool) {
		return kind, message, true
	}
}

var (
	regStatusCode       = regexp.MustCompile(`statuscode: (\d+)`)
	regEvictionResource = regexp.MustCompile(`low on resource: ([\w-]+)`)
)

func translateFailedScheduling(note string) (WorkloadWarningEventKind, string, bool) {
	return WorkloadWarningEventKindPlacementFailed, translate.SchedulingFailure(note), true
}

func translateFailedMount(note string) (WorkloadWarningEventKind, string, bool) {
	if msg, ok := translate.MissingSource(note); ok {
		return WorkloadWarningEventKindMountFailed, "Failed to mount files into the instance. " + msg, true
	}
	return WorkloadWarningEventKindMountFailed, "Failed to mount files or storage into the instance.", true
}

func translateFailed(note string) (WorkloadWarningEventKind, string, bool) {
	switch {
	case strings.HasPrefix(note, "Failed to pull image"):
		return WorkloadWarningEventKindImagePullFailed, "Failed to download the container image. Check that the image exists and is accessible.", true
	case note == "Error: ErrImagePull", note == "Error: ImagePullBackOff":
		// Follow-up events for an image download failure that has already been reported.
		return "", "", false
	case strings.HasPrefix(note, "Error: "):
		if msg, ok := translate.MissingSource(note); ok {
			return WorkloadWarningEventKindStartFailed, "The instance failed to start. " + msg, true
		}
		return WorkloadWarningEventKindStartFailed, "The instance failed to start.", true
	}
	return "", "", false
}

func translateBackOff(note string) (WorkloadWarningEventKind, string, bool) {
	if strings.HasPrefix(note, "Back-off restarting failed container") {
		return WorkloadWarningEventKindCrashLoop, "The instance keeps crashing and is restarted with increasing delay. Check the logs for details.", true
	}
	// Image download back-off is already covered by the image download failure.
	return "", "", false
}

func translateUnhealthy(note string) (WorkloadWarningEventKind, string, bool) {
	var check, consequence string
	switch {
	case strings.HasPrefix(note, "Readiness probe"):
		check, consequence = "Readiness", "The instance does not receive traffic while failing."
	case strings.HasPrefix(note, "Liveness probe"):
		check, consequence = "Liveness", "The instance is restarted if it keeps failing."
	case strings.HasPrefix(note, "Startup probe"):
		check, consequence = "Startup", "The instance is restarted if it does not start in time."
	default:
		return "", "", false
	}

	detail := ""
	switch {
	case regStatusCode.MatchString(note):
		detail = fmt.Sprintf(": responded with status code %s", regStatusCode.FindStringSubmatch(note)[1])
	case strings.Contains(note, "connection refused"):
		detail = ": the connection was refused"
	case strings.Contains(note, "deadline exceeded"), strings.Contains(note, "timeout"), strings.Contains(note, "timed out"):
		detail = ": timed out"
	}
	return WorkloadWarningEventKindHealthCheckFailed, fmt.Sprintf("%s check failed%s. %s", check, detail, consequence), true
}

func translateEvicted(note string) (WorkloadWarningEventKind, string, bool) {
	if strings.Contains(note, "ephemeral local storage usage exceeds") {
		return WorkloadWarningEventKindInstanceStopped, "The instance was stopped because it used more disk space than allowed.", true
	}
	if m := regEvictionResource.FindStringSubmatch(note); m != nil {
		resource := m[1]
		switch resource {
		case "ephemeral-storage":
			resource = "disk space"
		case "memory":
		default:
			resource = "resources"
		}
		return WorkloadWarningEventKindInstanceStopped, fmt.Sprintf("The instance was stopped because the server ran low on %s.", resource), true
	}
	return WorkloadWarningEventKindInstanceStopped, "The instance was stopped to free up resources on the server.", true
}

func translateFailedCreate(note string) (WorkloadWarningEventKind, string, bool) {
	switch {
	case strings.Contains(note, "exceeded quota"):
		return WorkloadWarningEventKindInstanceCreationFailed, "New instances could not be created because the team's resource quota is exceeded.", true
	case strings.Contains(note, "violates PodSecurity"):
		return WorkloadWarningEventKindInstanceCreationFailed, "New instances were rejected by the platform's security policy.", true
	case strings.Contains(note, "admission webhook") && strings.Contains(note, "denied the request"):
		return WorkloadWarningEventKindInstanceCreationFailed, "New instances were rejected by a platform policy.", true
	}
	return WorkloadWarningEventKindInstanceCreationFailed, "New instances could not be created.", true
}

func translateFailedMetrics(string) (WorkloadWarningEventKind, string, bool) {
	return WorkloadWarningEventKindScalingFailed, "Automatic scaling could not read the metrics it needs, so the number of instances is not adjusted.", true
}

func translateFailedRescale(string) (WorkloadWarningEventKind, string, bool) {
	return WorkloadWarningEventKindScalingFailed, "Automatic scaling failed to change the number of instances.", true
}

func toWarningEvent(base WorkloadEventBase, reason string, raw []byte) (*WorkloadWarningEvent, error) {
	var data warningEventData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	kind, message, ok := translateWarning(reason, data.Note)
	if !ok {
		return nil, fmt.Errorf("unsupported workload warning event: %q", reason)
	}
	base.Message = message
	event := &WorkloadWarningEvent{
		WorkloadEventBase: base,
		Kind:              kind,
		Count:             max(data.Count, 1),
		FirstSeenAt:       data.FirstSeen,
	}
	if data.Instance != "" {
		event.Instance = &data.Instance
	}
	if event.FirstSeenAt.IsZero() || event.FirstSeenAt.After(base.TriggeredAt) {
		event.FirstSeenAt = base.TriggeredAt
	}
	return event, nil
}

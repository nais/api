package activitylog

import (
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sirupsen/logrus"
)

type filter struct {
	action       ActivityLogEntryAction
	resourceType []ActivityLogEntryResourceType
}

var knownFilters = map[ActivityLogActivityType]filter{}

// reverseFilters maps "resource_type:action" strings to their ActivityLogActivityType values.
var reverseFilters = map[string][]ActivityLogActivityType{}

// WebhookEventTypeInfo describes a single webhook-subscribable event type.
type WebhookEventTypeInfo struct {
	// Type is the identifier used in webhook subscription event_types (e.g. "TEAM_MEMBER_ADDED").
	Type ActivityLogActivityType `json:"type"`
	// CloudEventType is the CloudEvents-spec type string (e.g. "io.nais.team.member.added").
	CloudEventType string `json:"cloudEventType"`
	// Description is a human-readable summary of the event.
	Description string `json:"description"`
	// Group is a logical grouping label for the event type (e.g. "Team", "Service Account").
	Group string `json:"group"`
	// TeamScoped indicates if this event type can be subscribed to by team-scoped webhooks.
	TeamScoped bool `json:"teamScoped"`

	ignoreWebhook bool // internal flag to indicate that this event type should not be exposed in the webhook catalogue
}

type ActivityTypeOption func(*WebhookEventTypeInfo)

// WithDescription sets a custom description for the event type.
func WithDescription(desc string) ActivityTypeOption {
	return func(info *WebhookEventTypeInfo) {
		info.Description = desc
	}
}

// WithGroup sets a custom group label for the event type.
func WithGroup(group string) ActivityTypeOption {
	return func(info *WebhookEventTypeInfo) {
		info.Group = group
	}
}

// GlobalOnly marks the event type as global-only (not team-scoped).
func GlobalOnly() ActivityTypeOption {
	return func(info *WebhookEventTypeInfo) {
		info.TeamScoped = false
	}
}

// IgnoreWebhook excludes the event type from the webhook event type catalogue entirely.
func IgnoreWebhook() ActivityTypeOption {
	return func(info *WebhookEventTypeInfo) {
		info.ignoreWebhook = true
	}
}

// eventTypeInfos stores metadata for all registered activity types.
var eventTypeInfos = map[ActivityLogActivityType]WebhookEventTypeInfo{}

// groupPrefixes maps known multi-word prefixes to their display group names.
var groupPrefixes = map[string]string{
	"SERVICE_ACCOUNT":             "Service Account",
	"GENERIC_KUBERNETES_RESOURCE": "Kubernetes",
	"OPENSEARCH":                  "OpenSearch",
	"JOB_RUN":                     "Job",
}

// autoGroupAndDescription derives a display group and description from an activity type name.
// Example: "TEAM_MEMBER_ADDED" → group "Team", description "Team member added".
func autoGroupAndDescription(at ActivityLogActivityType) (description, group string) {
	s := string(at)

	// Check multi-word prefix overrides first (longest match wins)
	longestPrefix := ""
	longestGroup := ""
	for prefix, grp := range groupPrefixes {
		if (s == prefix || strings.HasPrefix(s, prefix+"_")) && len(prefix) > len(longestPrefix) {
			longestPrefix = prefix
			longestGroup = grp
		}
	}
	if longestGroup != "" {
		group = longestGroup
	} else {
		// Single-word group: first token, title-cased
		idx := strings.Index(s, "_")
		if idx < 0 {
			group = titleCase(s)
		} else {
			group = titleCase(s[:idx])
		}
	}

	words := strings.Split(strings.ToLower(s), "_")
	description = strings.Join(words, " ")
	// Title-case first word only for sentence-style description
	if len(description) > 0 {
		description = strings.ToUpper(description[:1]) + description[1:]
	}
	return
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

// CloudEventType converts an ActivityLogActivityType to a CloudEvents-spec type string.
// Example: "TEAM_MEMBER_ADDED" → "io.nais.team.member.added".
func CloudEventType(at ActivityLogActivityType) string {
	lower := strings.ToLower(string(at))
	dotted := strings.ReplaceAll(lower, "_", ".")
	return "io.nais." + dotted
}

// KnownEventTypes returns metadata for all registered activity log event types, suitable
// for exposing as a webhook event type catalogue.
func KnownEventTypes() []WebhookEventTypeInfo {
	result := make([]WebhookEventTypeInfo, 0, len(knownFilters))
	for at := range knownFilters {
		info, ok := eventTypeInfos[at]
		if !ok {
			logrus.WithField("activity_type", at).Warn("activity type registered without webhook event type info; using auto-generated description and group")
			desc, grp := autoGroupAndDescription(at)
			info = WebhookEventTypeInfo{
				Type:           at,
				CloudEventType: CloudEventType(at),
				Description:    desc,
				Group:          grp,
				TeamScoped:     true,
			}
		}

		if info.ignoreWebhook {
			continue
		}
		result = append(result, info)
	}
	slices.SortFunc(result, func(a, b WebhookEventTypeInfo) int {
		if a.Group != b.Group {
			return strings.Compare(a.Group, b.Group)
		}
		return strings.Compare(string(a.Type), string(b.Type))
	})
	return result
}

// IsTeamScoped returns true if the event type is subscribable by team webhooks.
func IsTeamScoped(at ActivityLogActivityType) bool {
	info, ok := eventTypeInfos[at]
	if !ok {
		return true
	}
	return info.TeamScoped
}

// IsValidActivityType returns true if the event type is '*' or a registered activity type.
func IsValidActivityType(at string) bool {
	if at == "*" {
		return true
	}
	_, ok := knownFilters[ActivityLogActivityType(at)]
	return ok
}

// RegisterActivityType registers an activity log activity type, configuring its action,
// resourceType mapping, and optional webhook options.
func RegisterActivityType(activityType ActivityLogActivityType, action ActivityLogEntryAction, resourceType ActivityLogEntryResourceType, opts ...ActivityTypeOption) {
	if f, ok := knownFilters[activityType]; ok {
		if f.action == action {
			f.resourceType = append(f.resourceType, resourceType)
			slices.Sort(f.resourceType)
			f.resourceType = slices.Compact(f.resourceType)

			knownFilters[activityType] = f
			rebuildReverseFilters()
		} else {
			panic("activity type already registered: " + string(activityType) + " with action " + string(f.action))
		}
	} else {
		knownFilters[activityType] = filter{
			action:       action,
			resourceType: []ActivityLogEntryResourceType{resourceType},
		}
		rebuildReverseFilters()
	}

	desc, grp := autoGroupAndDescription(activityType)
	info := &WebhookEventTypeInfo{
		Type:           activityType,
		CloudEventType: CloudEventType(activityType),
		Description:    desc,
		Group:          grp,
		TeamScoped:     true,
	}

	for _, opt := range opts {
		opt(info)
	}

	eventTypeInfos[activityType] = *info
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

func withFrom(filter *ActivityLogFilter) pgtype.Timestamptz {
	if filter == nil || filter.From == nil {
		return pgtype.Timestamptz{Valid: false}
	}

	return pgtype.Timestamptz{Time: *filter.From, Valid: true}
}

func withTo(filter *ActivityLogFilter) pgtype.Timestamptz {
	if filter == nil || filter.To == nil {
		return pgtype.Timestamptz{Valid: false}
	}

	return pgtype.Timestamptz{Time: *filter.To, Valid: true}
}

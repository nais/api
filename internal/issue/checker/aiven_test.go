package checker

import (
	"context"
	"testing"

	"github.com/aiven/go-client-codegen/handler/project"
	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/thirdparty/aiven"
	"github.com/sirupsen/logrus/hooks/test"
)

func TestAivenIssuesUseResourceNames(t *testing.T) {
	for _, tt := range []struct {
		name         string
		serviceType  string
		serviceName  string
		team         string
		resource     string
		event        string
		message      string
		severity     string
		want         issue.Severity
		issueType    issue.IssueType
		resourceType issue.ResourceType
	}{
		{
			name:        "opensearch with hyphenated resource name",
			serviceType: "opensearch", serviceName: "opensearch-navno-enonic-cms-archive",
			team: "navno", resource: "enonic-cms-archive",
			event:    "user_alert_resource_usage_disk",
			message:  "Your opensearch service enonic-cms-archive is low on disk space. Write operations will be denied and other functionality may become unavailable.",
			severity: "critical", want: issue.SeverityCritical,
			issueType: issue.IssueTypeOpenSearch, resourceType: issue.ResourceTypeOpensearch,
		},
		{
			name:        "valkey with hyphenated resource name",
			serviceType: "valkey", serviceName: "valkey-navno-session-cache",
			team: "navno", resource: "session-cache",
			event:    "service_event",
			message:  "Your valkey service session-cache reports: service_event",
			severity: "warning", want: issue.SeverityWarning,
			issueType: issue.IssueTypeValkey, resourceType: issue.ResourceTypeValkey,
		},
		{
			name:        "single segment resource name",
			serviceType: "valkey", serviceName: "valkey-myteam-cache",
			team: "myteam", resource: "cache",
			event:    "service_event",
			message:  "Your valkey service cache reports: service_event",
			severity: "info", want: issue.SeverityTodo,
			issueType: issue.IssueTypeValkey, resourceType: issue.ResourceTypeValkey,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			log, _ := test.NewNullLogger()
			check := Aiven{
				aivenClient: &alertsAivenClient{alerts: []project.AlertOut{{
					ServiceName: &tt.serviceName,
					ServiceType: &tt.serviceType,
					Event:       tt.event,
					Severity:    tt.severity,
				}}},
				tenant:       "nav",
				environments: []string{"prod-gcp"},
				log:          log,
			}
			got, err := check.Run(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			want := []Issue{{
				ResourceName: tt.resource,
				ResourceType: tt.resourceType,
				Env:          "prod-gcp",
				Team:         tt.team,
				IssueType:    tt.issueType,
				Message:      tt.message,
				Severity:     tt.want,
				IssueDetails: issue.AivenIssueDetails{Event: tt.event},
			}}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("issues mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

type alertsAivenClient struct {
	aiven.AivenClient
	alerts []project.AlertOut
}

func (c *alertsAivenClient) ProjectAlertsList(context.Context, string) ([]project.AlertOut, error) {
	return c.alerts, nil
}

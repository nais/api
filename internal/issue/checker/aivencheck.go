package checker

import (
	"context"
	"strings"

	aiven "github.com/aiven/go-client-codegen"
)

type Aiven struct {
	AivenClient aiven.Client
	Projects    []string
}

type AivenIssueDetails struct {
	Message string `json:"message"`
}

func (a Aiven) Run(ctx context.Context) ([]Issue, error) {
	ret := make([]Issue, 0)
	for _, project := range a.Projects {
		alerts, err := a.AivenClient.ProjectAlertsList(ctx, project)
		if err != nil {
			return nil, err
		}

		mapAlerts := make(map[string]Issue)

		for _, alert := range alerts {
			key := *alert.ServiceType + "-" + *alert.ServiceName + "-" + alert.Event
			issue := Issue{
				ResourceName: *alert.ServiceName,
				ResourceType: *alert.ServiceType, // TODO: assume these are ok, may have to map
				Env:          project,
				Team:         getTeamFromServiceName(*alert.ServiceName), // lookup team by project and service type and name
				IssueType:    IssueTypeAivenIssue,
				IssueDetails: AivenIssueDetails{
					Message: alert.Event,
				},
				Severity: severity(alert.Severity),
			}
			mapAlerts[key] = issue
		}
		for _, issue := range mapAlerts {
			ret = append(ret, issue)
		}
	}

	return ret, nil
}

func getTeamFromServiceName(s string) string {
	parts := strings.Split(s, "-")
	if len(parts) >= 3 {
		return parts[1]
	}
	return "unknown"
}

func severity(severity string) Severity {
	switch severity {
	case "critical":
		return SeverityCritical
	case "warning":
		return SeverityWarning
	default:
		return SeverityTodo
	}
}

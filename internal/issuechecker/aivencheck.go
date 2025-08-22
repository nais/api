package issuechecker

import (
	"context"

	aiven "github.com/aiven/go-client-codegen"
)

type AivenCheck struct {
	AivenClient aiven.Client
	Projects    []string
}

type AivenAlert struct {
	Message string `json:"message"`
}

func (a AivenCheck) Run(ctx context.Context) ([]Issue, error) {
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
				Environment:  project,
				Team:         "unknown", // lookup team by project and service type and name
				Type:         "AivenAlert",
				IssueData:    AivenAlert{Message: alert.Event}, // TODO: map to something that makes sense to users
				Severity:     severity(alert.Severity),
			}
			mapAlerts[key] = issue
		}
		for _, issue := range mapAlerts {
			ret = append(ret, issue)
		}
	}

	return ret, nil
}

func severity(severity string) Severity {
	switch severity {
	case "critical":
		return SeverityError
	case "warning":
		return SeverityWarning
	default:
		return SeverityTodo
	}
}

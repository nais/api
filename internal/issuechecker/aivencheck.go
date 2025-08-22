package issuechecker

import (
	"context"

	aiven "github.com/aiven/go-client-codegen"
)

type AivenCheck struct {
	AivenClient aiven.Client
	Projects    []string
}

func (a AivenCheck) Run(ctx context.Context) ([]Issue, error) {
	ret := make([]Issue, 0)
	for _, project := range a.Projects {
		alerts, err := a.AivenClient.ProjectAlertsList(ctx, project)
		if err != nil {
			return nil, err
		}

		for _, alert := range alerts {
			issue := Issue{
				ResourceName: *alert.ServiceName,
				ResourceType: *alert.ServiceType, // TODO: assume these are ok, may have to map
				Environment:  project,
				Team:         "unknown", // lookup team by project and service type and name
				Type:         "AivenError",
				IssueData:    alert,
				Severity:     severity(alert.Severity),
			}

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

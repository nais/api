package checker

import (
	"context"
	"strings"

	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/thirdparty/aivencache"
	"github.com/sirupsen/logrus"
)

type Aiven struct {
	AivenClient  aivencache.AivenClient
	Tenant       string
	Environments []string
}

type AivenIssueDetails struct {
	Message string `json:"message"`
}

func (a Aiven) Run(ctx context.Context) ([]Issue, error) {
	ret := make([]Issue, 0)

	for _, p := range projects(a.Tenant, a.Environments) {
		logrus.WithField("issues", "aiven").Infof("listing aiven alerts for project %s\n", p)
		alerts, err := a.AivenClient.ProjectAlertsList(ctx, p)
		if err != nil {
			logrus.WithError(err).WithField("issues", "aiven").Errorf("failed listing aiven alerts for project %s", p)
			continue
		}

		mapAlerts := make(map[string]Issue)

		for _, alert := range alerts {
			key := *alert.ServiceType + "-" + *alert.ServiceName + "-" + alert.Event
			issue := Issue{
				ResourceName: *alert.ServiceName,
				ResourceType: *alert.ServiceType, // TODO: assume these are ok, may have to map
				Env:          p,
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

func projects(tenant string, envs []string) []string {
	ret := []string{}
	for _, env := range envs {
		if tenant == "nav" && strings.HasSuffix(env, "-fss") {
			continue
		}
		ret = append(ret, tenant+"-"+environmentmapper.ClusterName(env))
	}
	return ret
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

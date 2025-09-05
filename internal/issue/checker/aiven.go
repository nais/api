package checker

import (
	"context"
	"strings"

	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/thirdparty/aivencache"
	"github.com/sirupsen/logrus"
)

type Aiven struct {
	aivenClient  aivencache.AivenClient
	tenant       string
	environments []string
	log          logrus.FieldLogger
}

func (a Aiven) Run(ctx context.Context) ([]Issue, error) {
	ret := make([]Issue, 0)

	for env, p := range projects(a.tenant, a.environments) {
		a.log.WithField("project", p).Debug("listing Aiven alerts for project")
		alerts, err := a.aivenClient.ProjectAlertsList(ctx, p)
		if err != nil {
			a.log.WithError(err).WithField("project", p).Error("failed listing Aiven alerts for project")
			continue
		}

		mapAlerts := make(map[string]Issue)

		for _, alert := range alerts {
			key := *alert.ServiceType + "-" + *alert.ServiceName + "-" + alert.Event
			issueType := issueTypeFromServiceType(*alert.ServiceType)
			if issueType == "" {
				a.log.WithField("service_type", *alert.ServiceType).WithField("event", alert.Event).Warn("unknown Aiven service type")
				continue
			}
			issue := Issue{
				ResourceName: *alert.ServiceName,
				ResourceType: issue.ResourceType(strings.ToUpper(*alert.ServiceType)), // TODO: assume these are ok, may have to map
				Env:          env,
				Team:         getTeamFromServiceName(*alert.ServiceName), // lookup team by project and service type and name
				IssueType:    issueType,
				Message:      alert.Event, // TODO: a separate message?
				IssueDetails: issue.AivenIssueDetails{
					Event: alert.Event,
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

func issueTypeFromServiceType(serviceType string) issue.IssueType {
	for _, t := range issue.AllIssueType {
		if strings.EqualFold(string(t), serviceType) {
			return t
		}
	}
	return ""
}

func projects(tenant string, envs []string) map[string]string {
	ret := make(map[string]string)
	for _, env := range envs {
		if tenant == "nav" && strings.HasSuffix(env, "-fss") {
			continue
		}
		ret[env] = tenant + "-" + environmentmapper.ClusterName(env)
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

func severity(severity string) issue.Severity {
	switch severity {
	case "critical":
		return issue.SeverityCritical
	case "warning":
		return issue.SeverityWarning
	default:
		return issue.SeverityTodo
	}
}

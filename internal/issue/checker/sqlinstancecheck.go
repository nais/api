package checker

import (
	"context"
	"log"

	"github.com/nais/api/internal/persistence/sqlinstance"
	"github.com/nais/api/internal/team"
	"google.golang.org/api/sqladmin/v1"
)

type SQLInstance struct {
	SQLInstanceClient *sqladmin.InstancesService
	SQLInstanceLister KubernetesLister[*sqlinstance.SQLInstance]
}

type SQLInstanceLister struct{}

func (s *SQLInstanceLister) List(ctx context.Context, _ string) []*sqlinstance.SQLInstance {
	teams, err := team.ListAllSlugs(ctx)
	if err != nil {
		panic(err)
	}
	instances := make([]*sqlinstance.SQLInstance, 0)
	for _, team := range teams {
		sqlInstances := sqlinstance.ListAllForTeam(ctx, team)
		for _, instance := range sqlInstances {
			instances = append(instances, &sqlinstance.SQLInstance{
				Name:            instance.Name,
				ProjectID:       instance.ProjectID,
				EnvironmentName: instance.EnvironmentName,
				TeamSlug:        team,
			})
		}
	}
	return instances
}

type SQLInstanceIssueDetails struct {
	State   string `json:"state"`
	Message string `json:"message"`
}

func (s SQLInstance) Run(ctx context.Context) ([]Issue, error) {
	ret := make([]Issue, 0)

	for _, instance := range s.SQLInstanceLister.List(ctx, "TODO") {
		i, err := s.SQLInstanceClient.Get(instance.ProjectID, instance.Name).Context(ctx).Do()
		if err != nil {
			return nil, err
		}
		if i.State == "RUNNABLE" && i.Settings.ActivationPolicy == "ALWAYS" {
			log.Printf("Skipping instance %s in project %s, state is RUNNABLE and activation policy is ALWAYS", instance.Name, instance.ProjectID)
			continue
		}
		state, message, severity := parseState(i.State, i.Settings.ActivationPolicy)
		ret = append(ret, Issue{
			ResourceName: instance.Name,
			ResourceType: "sqlinstance",
			Env:          instance.EnvironmentName,
			Team:         instance.TeamSlug.String(),
			IssueType:    IssueTypeSQLInstanceIssue,

			IssueDetails: SQLInstanceIssueDetails{
				State:   state,
				Message: message,
			},
			Severity: severity,
		})
	}

	return ret, nil
}

func parseState(state, ap string) (string, string, Severity) {
	type compound struct {
		severity, message string
	}
	lookup := map[string]compound{
		"SQL_INSTANCE_STATE_UNSPECIFIED": {severity: string(SeverityCritical), message: "The state of the instance is unknown."},
		"SUSPENDED":                      {severity: string(SeverityCritical), message: "The instance is not available, for example due to problems with billing."},
		"PENDING_DELETE":                 {severity: string(SeverityWarning), message: "The instance is being deleted."},
		"PENDING_CREATE":                 {severity: string(SeverityWarning), message: "The instance is being created."},
		"MAINTENANCE":                    {severity: string(SeverityCritical), message: "The instance is down for maintenance."},
		"FAILED":                         {severity: string(SeverityCritical), message: "The creation of the instance failed or a fatal error occurred during maintenance."},
		"REPAIRING":                      {severity: string(SeverityWarning), message: "(Applicable to read pool nodes only.) The read pool node needs to be repaired. The database might be unavailable."},
		"STOPPED":                        {severity: string(SeverityCritical), message: "The instance has been stopped"},
	}
	if state == "RUNNABLE" && ap != "ALWAYS" {
		state = "STOPPED"
	}
	if s, found := lookup[state]; found {
		return state, s.message, Severity(s.severity)
	}
	return "UNKNOWN", "Unknown state", SeverityCritical
}

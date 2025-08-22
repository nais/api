package issuechecker

import (
	"context"
	"log"

	"github.com/nais/api/internal/persistence/sqlinstance"
	"github.com/nais/api/internal/team"
	"google.golang.org/api/sqladmin/v1"
)

type SQLInstanceCheck struct {
	SQLInstanceClient *sqladmin.InstancesService
	SQLInstanceLister KubernetesLister[*sqlinstance.SQLInstance]
}

type SQLInstanceLister struct{}

func (s *SQLInstanceLister) List(ctx context.Context) []*sqlinstance.SQLInstance {
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

type SQLInstanceState struct {
	State            string `json:"state"`
	ActivationPolicy string `json:"activationPolicy"`
}

func (s SQLInstanceCheck) Run(ctx context.Context) ([]Issue, error) {
	ret := make([]Issue, 0)

	for _, instance := range s.SQLInstanceLister.List(ctx) {
		i, err := s.SQLInstanceClient.Get(instance.ProjectID, instance.Name).Context(ctx).Do()
		if err != nil {
			return nil, err
		}
		if i.State == "RUNNABLE" && i.Settings.ActivationPolicy == "ALWAYS" {
			log.Printf("Skipping instance %s in project %s, state is RUNNABLE and activation policy is ALWAYS", instance.Name, instance.ProjectID)
			continue
		}
		ret = append(ret, Issue{
			ResourceName: instance.Name,
			ResourceType: "sqlinstance",
			Environment:  instance.EnvironmentName,
			Team:         instance.TeamSlug.String(),
			Type:         "SQLInstanceState",
			IssueData: SQLInstanceState{
				State:            i.State,
				ActivationPolicy: i.Settings.ActivationPolicy,
			},
			Severity: SeverityError,
		})
	}

	return ret, nil
}

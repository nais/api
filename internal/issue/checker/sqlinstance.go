package checker

import (
	"context"
	"runtime"
	"slices"
	"sync"

	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/kubernetes/watchers"
	"github.com/nais/api/internal/persistence/sqlinstance"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/sqladmin/v1"
)

var deprecatedVersions = []string{
	"POSTGRES_12",
	"POSTGRES_13",
}

type SQLInstance struct {
	Client            *sqlinstance.Client
	SQLInstanceLister KubernetesLister[*sqlinstance.SQLInstance]
	Log               logrus.FieldLogger
}

type sqlInstanceLister struct {
	watcher *watchers.SqlInstanceWatcher
}

func (s *sqlInstanceLister) List(ctx context.Context) []*sqlinstance.SQLInstance {
	instances := make([]*sqlinstance.SQLInstance, 0)
	for _, instance := range s.watcher.All() {
		instances = append(instances, &sqlinstance.SQLInstance{
			Name:            instance.Obj.Name,
			ProjectID:       instance.Obj.ProjectID,
			EnvironmentName: instance.Obj.EnvironmentName,
			TeamSlug:        instance.Obj.TeamSlug,
		})
	}
	return instances
}

func (s *sqlInstanceLister) Get(env, namespace, name string) (*sqlinstance.SQLInstance, error) {
	return s.Get(env, namespace, name)
}

func (s SQLInstance) Run(ctx context.Context) ([]Issue, error) {
	ret := make([]Issue, 0)

	instances := s.instances(ctx)

	for instance, i := range instances {
		if slices.Contains(deprecatedVersions, i.DatabaseVersion) {
			ret = append(ret, Issue{
				ResourceName: instance.Name,
				ResourceType: issue.ResourceTypeSQLInstance,
				Env:          instance.EnvironmentName,
				Team:         instance.TeamSlug.String(),
				IssueType:    issue.IssueTypeSqlInstanceVersion,
				Message:      "The instance is running a deprecated version of PostgreSQL: " + i.DatabaseVersion,
				Severity:     issue.SeverityWarning,
			})
		}

		if i.State == "RUNNABLE" && i.Settings.ActivationPolicy == "ALWAYS" {
			s.Log.Debugf("skipping instance %s in project %s, state is RUNNABLE and activation policy is ALWAYS", instance.Name, instance.ProjectID)
			continue
		}
		state, message, severity := parseState(i.State, i.Settings.ActivationPolicy)
		ret = append(ret, Issue{
			ResourceName: instance.Name,
			ResourceType: issue.ResourceTypeSQLInstance,
			Env:          instance.EnvironmentName,
			Team:         instance.TeamSlug.String(),
			IssueType:    issue.IssueTypeSqlInstanceState,
			Message:      message,
			IssueDetails: issue.SQLInstanceIssueDetails{
				State:   state,
				Message: message,
			},
			Severity: severity,
		})
	}

	return ret, nil
}

// instances fetches all instances in parallel from the Google API mapping them to the corresponding SQLInstance
func (s SQLInstance) instances(ctx context.Context) map[*sqlinstance.SQLInstance]sqladmin.DatabaseInstance {
	ret := make(map[*sqlinstance.SQLInstance]sqladmin.DatabaseInstance)

	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, runtime.NumCPU())

	for _, instance := range s.SQLInstanceLister.List(ctx) {
		wg.Add(1)
		go func(instance *sqlinstance.SQLInstance) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			i, err := s.Client.Admin.GetInstance(ctx, instance.ProjectID, instance.Name)
			if err != nil {
				s.Log.WithError(err).WithField("instance", instance.Name).Error("getting sqlinstance")
				return
			}
			mu.Lock()
			ret[instance] = *i
			mu.Unlock()
		}(instance)
	}
	wg.Wait()
	return ret
}

func parseState(state, ap string) (string, string, issue.Severity) {
	type compound struct {
		severity issue.Severity
		message  string
	}
	lookup := map[string]compound{
		"SQL_INSTANCE_STATE_UNSPECIFIED": {severity: issue.SeverityCritical, message: "The state of the instance is unknown."},
		"SUSPENDED":                      {severity: issue.SeverityCritical, message: "The instance is not available, for example due to problems with billing."},
		"PENDING_DELETE":                 {severity: issue.SeverityWarning, message: "The instance is being deleted."},
		"PENDING_CREATE":                 {severity: issue.SeverityWarning, message: "The instance is being created."},
		"MAINTENANCE":                    {severity: issue.SeverityCritical, message: "The instance is down for maintenance."},
		"FAILED":                         {severity: issue.SeverityCritical, message: "The creation of the instance failed or a fatal error occurred during maintenance."},
		"STOPPED":                        {severity: issue.SeverityCritical, message: "The instance has been stopped."},
	}
	if state == "RUNNABLE" && ap != "ALWAYS" {
		state = "STOPPED"
	}
	if state == "SQL_INSTANCE_STATE_UNSPECIFIED" {
		state = "UNSPECIFIED"
	}
	if s, found := lookup[state]; found {
		return state, s.message, s.severity
	}
	return "UNKNOWN", "Unknown state", issue.SeverityCritical
}

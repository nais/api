package status2

import (
	"context"
	"fmt"
	"time"

	aiven "github.com/aiven/go-client-codegen"
	"github.com/nais/api/internal/persistence/opensearch"
	"github.com/nais/api/internal/persistence/sqlinstance"
	"google.golang.org/api/sqladmin/v1"
)

var ResourceTypes = []string{
	"opensearch",
}

type Client struct {
	aivenClient       aiven.Client
	sqlInstanceClient *sqladmin.InstancesService
	info              Info
}

type Message struct {
	Created  time.Time
	Event    string
	Severity Severity
}

type Severity string

type Status struct {
	ResourceType string
	Name         string
	Environment  string
	Team         string
	Messages     []*Message
	State        State
}

type State string

type Info interface {
	OpenSearches(ctx context.Context) ([]*opensearch.OpenSearch, error)
	SQLInstances(ctx context.Context) ([]sqlinstance.SQLInstance, error)
}

func (c *Client) Run(ctx context.Context) ([]*Status, error) {
	statuses := make([]*Status, 0)

	openSearches, err := c.info.OpenSearches(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OpenSearches: %w", err)
	}
	for _, os := range openSearches {
		s, err := c.AivenStatus(ctx, os.AivenProject, os.Name, "opensearch")
		if err != nil {
			return nil, fmt.Errorf("failed to get Aiven status for OpenSearch %s: %w", os.Name, err)
		}
		s.Environment = os.EnvironmentName
		s.Team = os.TeamSlug.String()
		statuses = append(statuses, s)
	}

	sqlInstances, err := c.info.SQLInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get SQL instances: %w", err)
	}
	for _, sqlInstance := range sqlInstances {
		s, err := c.CloudSQLStatus(ctx, sqlInstance.ProjectID, sqlInstance.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to get Cloud SQL status for instance %s: %w", sqlInstance.Name, err)
		}
		s.Environment = sqlInstance.EnvironmentName
		s.Team = sqlInstance.TeamSlug.String()
		statuses = append(statuses, s)
	}

	return statuses, nil
}

func (c *Client) CloudSQLStatus(ctx context.Context, project, name string) (*Status, error) {
	instance, err := c.sqlInstanceClient.Get(project, name).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get Cloud SQL instance %s: %w", name, err)
	}
	ret := &Status{
		ResourceType: "sqlinstance",
		Name:         name,
		State:        sqlStatus(instance),
	}

	for _, sr := range instance.SuspensionReason {
		ret.Messages = append(ret.Messages, &Message{
			Created:  time.Now(),
			Event:    sr,
			Severity: Severity("critical"),
		})
	}

	return ret, nil
}

func sqlStatus(instance *sqladmin.DatabaseInstance) State {
	if instance.Settings.ActivationPolicy == "NEVER" {
		return State("STOPPED")
	}
	return State(instance.State)
}

func (c *Client) AivenStatus(ctx context.Context, project, name, resourceType string) (*Status, error) {
	s, err := c.aivenClient.ServiceGet(ctx, project, name)
	if err != nil {
		return nil, err
	}
	ret := &Status{
		ResourceType: resourceType,
		Name:         name,
		State:        State(s.State),
	}

	sa, err := c.aivenClient.ServiceAlertsList(ctx, project, name)
	if err != nil {
		return nil, err
	}
	for _, a := range sa {
		ret.Messages = append(ret.Messages, &Message{
			Created:  a.CreateTime,
			Event:    fmt.Sprintf("%v on %v", a.Event, *a.NodeName),
			Severity: Severity(a.Severity),
		})
	}
	return ret, nil
}
func NewClient(ctx context.Context, token string, info Info) (*Client, error) {
	c, err := aiven.NewClient(aiven.TokenOpt(token), aiven.UserAgentOpt("nais-api"))
	if err != nil {
		return nil, err
	}

	admin, err := sqladmin.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create SQL Admin service: %w", err)
	}

	return &Client{aivenClient: c, info: info, sqlInstanceClient: admin.Instances}, nil
}

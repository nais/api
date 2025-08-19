package status2

import (
	"context"
	"fmt"
	"time"

	aiven "github.com/aiven/go-client-codegen"
)

type Client struct {
	aivenClient aiven.Client
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
	Messages     []*Message
	State        State
}

type State string

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
func NewClient(ctx context.Context, token string) (*Client, error) {
	c, err := aiven.NewClient(aiven.TokenOpt(token), aiven.UserAgentOpt("nais-api"))
	if err != nil {
		return nil, err
	}
	return &Client{aivenClient: c}, nil
}

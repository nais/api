package slack

import (
	"context"

	"github.com/nais/api/internal/graph/model"
	"github.com/slack-go/slack"
)

// Slack is a client for sending messages to Slack
type Slack struct {
	feedbackChannel string
	client          *slack.Client
}

type SlackClient interface {
	PostMessage(channelName string, msgOptions []slack.MsgOption) (string, string, error)
	PostFeedbackMessage(msgOptions []slack.MsgOption) (string, string, error)
	GetFeedbackMessageOptions(ctx context.Context, tenant string, input model.CreateFeedbackInput) []slack.MsgOption
}

// New creates a new Slack client
func New(token string, feedbackChannel string) SlackClient {
	return &Slack{
		client:          slack.New(token),
		feedbackChannel: feedbackChannel,
	}
}

func (s *Slack) PostFeedbackMessage(msgOptions []slack.MsgOption) (string, string, error) {
	return s.PostMessage(s.feedbackChannel, msgOptions)
}

// SendMessage sends a message to a Slack channel
func (s *Slack) PostMessage(channelName string, msgOptions []slack.MsgOption) (string, string, error) {
	channelId, timestamp, err := s.client.PostMessage(channelName, msgOptions...)
	if err != nil {
		return "", "", err
	}
	return channelId, timestamp, nil
}

package feedback

import (
	"context"
	"strings"

	"github.com/slack-go/slack"
)

type client struct {
	tenant            string
	feedbackChannelID string
	slackClient       *slack.Client
}

type Client interface {
	PostFeedback(ctx context.Context, input *ReportConsoleUserFeedbackInput) error
}

func NewClient(tenant, apiToken, feedbackChannelID string) Client {
	return &client{
		tenant:            tenant,
		slackClient:       slack.New(apiToken),
		feedbackChannelID: feedbackChannelID,
	}
}

func (s *client) PostFeedback(ctx context.Context, input *ReportConsoleUserFeedbackInput) error {
	blocks := s.getFeedbackMessage(input)
	opt := slack.MsgOptionBlocks(blocks...)
	_, _, err := s.slackClient.PostMessageContext(ctx, s.feedbackChannelID, opt)
	return err
}

func (s *client) getFeedbackMessage(input *ReportConsoleUserFeedbackInput) []slack.Block {
	var headerText string
	switch input.Type {
	case ConsoleUserFeedbackTypeBug:
		headerText = ":bug: Bug report"
	case ConsoleUserFeedbackTypeChangeRequest:
		headerText = ":bulb: Change request"
	case ConsoleUserFeedbackTypeOther:
		headerText = ":speech_balloon: Other feedback"
	case ConsoleUserFeedbackTypeQuestion:
		headerText = ":question: Question"
	}

	details := []string{
		"*From:* " + input.Author,
		"*URL:* " + "https://console." + s.tenant + ".cloud.nais.io" + input.Path,
		"*Tenant:* " + s.tenant,
	}
	return []slack.Block{
		slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", headerText, true, false)),
		slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", strings.Join(details, "\n"), false, false), nil, nil),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(slack.NewTextBlockObject("plain_text", input.Feedback, false, false), nil, nil),
	}
}

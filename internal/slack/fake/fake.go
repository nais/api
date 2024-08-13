package fake

import "github.com/slack-go/slack"

type FakeSlackClient struct{}

func NewFakeSlackClient() *FakeSlackClient {
	return &FakeSlackClient{}
}

func (f *FakeSlackClient) PostMessage(channelName string, msgOptions []slack.MsgOption) (string, string, error) {
	return "", "", nil
}

func (f *FakeSlackClient) PostFeedbackMessage(msgOptions []slack.MsgOption) (string, string, error) {
	return "", "", nil
}

func (f *FakeSlackClient) PostComment(channelName, messageTs string, msgOptions []slack.MsgOption) error {
	return nil
}

func (f *FakeSlackClient) AddReaction(channelId, timestamp, reaction string) error {
	return nil
}

func (f *FakeSlackClient) GetFeedbackMessageOptions(tenant, user, uri, text string) []slack.MsgOption {
	return nil
}

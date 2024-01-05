package model_test

import (
	"testing"

	"github.com/nais/api/internal/graph/model"

	"github.com/stretchr/testify/assert"
)

func TestCreateTeamInput_Sanitize(t *testing.T) {
	input := model.CreateTeamInput{
		Purpose:      " some purpose ",
		SlackChannel: " #some-channel ",
	}
	sanitized := input.Sanitize()
	assert.Equal(t, "some purpose", sanitized.Purpose)
	assert.Equal(t, "#some-channel", sanitized.SlackChannel)

	assert.Equal(t, " some purpose ", input.Purpose)
	assert.Equal(t, " #some-channel ", input.SlackChannel)
}

func TestUdateTeamInput_Sanitize(t *testing.T) {
	input := model.UpdateTeamInput{
		Purpose:      ptr(" some purpose "),
		SlackChannel: ptr(" #some-channel "),
		SlackAlertsChannels: []*model.SlackAlertsChannelInput{
			{
				Environment: " foo ",
				ChannelName: ptr(" #foo "),
			},
			{
				Environment: " bar ",
				ChannelName: ptr(" #bar "),
			},
			{
				Environment: " baz ",
			},
		},
	}
	sanitized := input.Sanitize()
	assert.Equal(t, "some purpose", *sanitized.Purpose)
	assert.Equal(t, "#some-channel", *sanitized.SlackChannel)
	assert.Equal(t, "foo", sanitized.SlackAlertsChannels[0].Environment)
	assert.Equal(t, "#foo", *sanitized.SlackAlertsChannels[0].ChannelName)
	assert.Equal(t, "bar", sanitized.SlackAlertsChannels[1].Environment)
	assert.Equal(t, "#bar", *sanitized.SlackAlertsChannels[1].ChannelName)
	assert.Equal(t, "baz", sanitized.SlackAlertsChannels[2].Environment)
	assert.Nil(t, sanitized.SlackAlertsChannels[2].ChannelName)

	assert.Equal(t, " some purpose ", *input.Purpose)
	assert.Equal(t, " #some-channel ", *input.SlackChannel)
	assert.Equal(t, " foo ", input.SlackAlertsChannels[0].Environment)
	assert.Equal(t, " #foo ", *input.SlackAlertsChannels[0].ChannelName)
	assert.Equal(t, " bar ", input.SlackAlertsChannels[1].Environment)
	assert.Equal(t, " #bar ", *input.SlackAlertsChannels[1].ChannelName)
	assert.Equal(t, " baz ", input.SlackAlertsChannels[2].Environment)
	assert.Nil(t, input.SlackAlertsChannels[2].ChannelName)
}

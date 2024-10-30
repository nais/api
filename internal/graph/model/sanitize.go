package model

import (
	"strings"

	"k8s.io/utils/ptr"
)

func (input UpdateTeamSlackAlertsChannelInput) Sanitize() UpdateTeamSlackAlertsChannelInput {
	input.Environment = strings.TrimSpace(input.Environment)
	if input.ChannelName != nil {
		input.ChannelName = ptr.To(strings.TrimSpace(*input.ChannelName))
	}

	return input
}

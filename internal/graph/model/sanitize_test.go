package model_test

import (
	"testing"

	"github.com/nais/api/internal/graph/model"
	"k8s.io/utils/ptr"
)

func TestUpdateSlackAlertsChannelInput_Sanitize(t *testing.T) {
	input := model.UpdateTeamSlackAlertsChannelInput{
		ChannelName: ptr.To(" #some-channel "),
		Environment: " some-environment ",
	}
	sanitized := input.Sanitize()
	if expected := "#some-channel"; *sanitized.ChannelName != expected {
		t.Errorf("expected %q, got %q", expected, *sanitized.ChannelName)
	}

	if expected := " #some-channel "; *input.ChannelName != expected {
		t.Errorf("expected %q, got %q", expected, *input.ChannelName)
	}
}

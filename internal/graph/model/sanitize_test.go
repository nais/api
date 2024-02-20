package model_test

import (
	"testing"

	"github.com/nais/api/internal/graph/model"
	"k8s.io/utils/ptr"
)

func TestCreateTeamInput_Sanitize(t *testing.T) {
	input := model.CreateTeamInput{
		Purpose:      " some purpose ",
		SlackChannel: " #some-channel ",
	}
	sanitized := input.Sanitize()
	if expected := "some purpose"; sanitized.Purpose != expected {
		t.Errorf("expected %q, got %q", expected, sanitized.Purpose)
	}

	if expected := "#some-channel"; sanitized.SlackChannel != expected {
		t.Errorf("expected %q, got %q", expected, sanitized.SlackChannel)
	}

	if expected := " some purpose "; input.Purpose != expected {
		t.Errorf("expected %q, got %q", expected, input.Purpose)
	}

	if expected := " #some-channel "; input.SlackChannel != expected {
		t.Errorf("expected %q, got %q", expected, input.SlackChannel)
	}
}

func TestUdateTeamInput_Sanitize(t *testing.T) {
	input := model.UpdateTeamInput{
		Purpose:      ptr.To(" some purpose "),
		SlackChannel: ptr.To(" #some-channel "),
		SlackAlertsChannels: []*model.SlackAlertsChannelInput{
			{
				Environment: " foo ",
				ChannelName: ptr.To(" #foo "),
			},
			{
				Environment: " bar ",
				ChannelName: ptr.To(" #bar "),
			},
			{
				Environment: " baz ",
			},
		},
	}
	sanitized := input.Sanitize()

	if expected := "some purpose"; *sanitized.Purpose != expected {
		t.Errorf("expected %q, got %q", expected, *sanitized.Purpose)
	}

	if expected := "#some-channel"; *sanitized.SlackChannel != expected {
		t.Errorf("expected %q, got %q", expected, *sanitized.SlackChannel)
	}

	if expected := "foo"; sanitized.SlackAlertsChannels[0].Environment != expected {
		t.Errorf("expected %q, got %q", expected, sanitized.SlackAlertsChannels[0].Environment)
	}

	if expected := "#foo"; *sanitized.SlackAlertsChannels[0].ChannelName != expected {
		t.Errorf("expected %q, got %q", expected, *sanitized.SlackAlertsChannels[0].ChannelName)
	}

	if expected := "bar"; sanitized.SlackAlertsChannels[1].Environment != expected {
		t.Errorf("expected %q, got %q", expected, sanitized.SlackAlertsChannels[1].Environment)
	}

	if expected := "#bar"; *sanitized.SlackAlertsChannels[1].ChannelName != expected {
		t.Errorf("expected %q, got %q", expected, *sanitized.SlackAlertsChannels[1].ChannelName)
	}

	if expected := "baz"; sanitized.SlackAlertsChannels[2].Environment != expected {
		t.Errorf("expected %q, got %q", expected, sanitized.SlackAlertsChannels[2].Environment)
	}

	if sanitized.SlackAlertsChannels[2].ChannelName != nil {
		t.Errorf("expected nil, got %q", *sanitized.SlackAlertsChannels[2].ChannelName)
	}

	if expected := " some purpose "; *input.Purpose != expected {
		t.Errorf("expected %q, got %q", expected, *input.Purpose)
	}

	if expected := " #some-channel "; *input.SlackChannel != expected {
		t.Errorf("expected %q, got %q", expected, *input.SlackChannel)
	}

	if expected := " foo "; input.SlackAlertsChannels[0].Environment != expected {
		t.Errorf("expected %q, got %q", expected, input.SlackAlertsChannels[0].Environment)
	}

	if expected := " #foo "; *input.SlackAlertsChannels[0].ChannelName != expected {
		t.Errorf("expected %q, got %q", expected, *input.SlackAlertsChannels[0].ChannelName)
	}

	if expected := " bar "; input.SlackAlertsChannels[1].Environment != expected {
		t.Errorf("expected %q, got %q", expected, input.SlackAlertsChannels[1].Environment)
	}

	if expected := " #bar "; *input.SlackAlertsChannels[1].ChannelName != expected {
		t.Errorf("expected %q, got %q", expected, *input.SlackAlertsChannels[1].ChannelName)
	}

	if expected := " baz "; input.SlackAlertsChannels[2].Environment != expected {
		t.Errorf("expected %q, got %q", expected, input.SlackAlertsChannels[2].Environment)
	}

	if input.SlackAlertsChannels[2].ChannelName != nil {
		t.Errorf("expected nil, got %q", *input.SlackAlertsChannels[2].ChannelName)
	}
}

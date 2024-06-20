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

func TestUpdateTeamInput_Sanitize(t *testing.T) {
	input := model.UpdateTeamInput{
		Purpose:      ptr.To(" some purpose "),
		SlackChannel: ptr.To(" #some-channel "),
	}
	sanitized := input.Sanitize()

	if expected := "some purpose"; *sanitized.Purpose != expected {
		t.Errorf("expected %q, got %q", expected, *sanitized.Purpose)
	}

	if expected := "#some-channel"; *sanitized.SlackChannel != expected {
		t.Errorf("expected %q, got %q", expected, *sanitized.SlackChannel)
	}

	if expected := " some purpose "; *input.Purpose != expected {
		t.Errorf("expected %q, got %q", expected, *input.Purpose)
	}

	if expected := " #some-channel "; *input.SlackChannel != expected {
		t.Errorf("expected %q, got %q", expected, *input.SlackChannel)
	}
}

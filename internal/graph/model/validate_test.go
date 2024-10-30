package model_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/model"
	"k8s.io/utils/ptr"
)

func TestUpdateTeamInput_Validate(t *testing.T) {
	t.Run("valid data", func(t *testing.T) {
		input := model.UpdateTeamInput{
			Purpose:      ptr.To("valid purpose"),
			SlackChannel: ptr.To("#valid-channel"),
		}

		if err := input.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid purpose", func(t *testing.T) {
		input := model.UpdateTeamInput{
			Purpose: ptr.To(""),
		}

		if !errors.Is(input.Validate(), apierror.ErrTeamPurpose) {
			t.Fatalf("expected error %v, got %v", apierror.ErrTeamPurpose, input.Validate())
		}
	})

	t.Run("invalid slack channel", func(t *testing.T) {
		input := model.UpdateTeamInput{
			Purpose:      ptr.To("purpose"),
			SlackChannel: ptr.To("#a"),
		}
		err := input.Validate()
		if contains := "The Slack channel does not fit the requirements"; !strings.Contains(err.Error(), contains) {
			t.Errorf("expected error to contain %q, got %q", contains, err)
		}
	})
}

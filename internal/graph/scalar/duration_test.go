package scalar_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nais/api/internal/graph/scalar"
)

func TestUnmarshalDuration(t *testing.T) {
	t.Run("invalid type", func(t *testing.T) {
		_, err := scalar.UnmarshalDuration(123)
		if err.Error() != "input must be a string" {
			t.Errorf("expected error 'input must be a string', got %v", err)
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		_, err := scalar.UnmarshalDuration("123")
		if contains := "invalid duration format"; !strings.Contains(err.Error(), contains) {
			t.Errorf("expected error to contain %q, got %v", contains, err)
		}
	})

	t.Run("valid duration", func(t *testing.T) {
		d, err := scalar.UnmarshalDuration("2h45m")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if expected := 2*time.Hour + 45*time.Minute; d != expected {
			t.Errorf("expected duration %v, got %v", expected, d)
		}
	})
}

func TestMarshalDuration(t *testing.T) {
	marshaler := scalar.MarshalDuration(3*time.Hour + 30*time.Minute)

	var sb strings.Builder
	marshaler.MarshalGQL(&sb)
	if expected := `"3h30m0s"`; sb.String() != expected {
		t.Errorf("expected marshaled duration %s, got %s", expected, sb.String())
	}
}

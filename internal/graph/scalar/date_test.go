package scalar_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nais/api/internal/graph/scalar"
)

var tm = time.Date(2020, time.April, 20, 0, 0, 0, 0, time.UTC)

func TestDate_NewDate(t *testing.T) {
	date := scalar.NewDate(tm)
	if expected := "2020-04-20"; date.String() != expected {
		t.Errorf("expected %q, got %q", expected, date.String())
	}
}

func TestDate_MarshalGQLContext(t *testing.T) {
	date := scalar.NewDate(tm)
	buf := new(bytes.Buffer)
	if err := date.MarshalGQLContext(context.Background(), buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != `"2020-04-20"` {
		t.Errorf("expected %q, got %q", `"2020-04-20"`, buf.String())
	}
}

func TestDate_UnmarshalGQLContext(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid type", func(t *testing.T) {
		date := scalar.NewDate(tm)
		if expected, err := "date must be a string", date.UnmarshalGQLContext(ctx, 123); err.Error() != expected {
			t.Errorf("expected error %q, got %q", expected, err.Error())
		}
	})

	t.Run("invalid value", func(t *testing.T) {
		date := scalar.NewDate(tm)
		if contains, err := "", date.UnmarshalGQLContext(ctx, "foobar"); !strings.Contains(err.Error(), contains) {
			t.Errorf("expected error to contain %q, got %q", contains, err.Error())
		}
	})

	t.Run("empty string", func(t *testing.T) {
		date := scalar.NewDate(tm)
		if expected, err := "date must not be empty", date.UnmarshalGQLContext(ctx, ""); err.Error() != expected {
			t.Errorf("expected error %q, got %q", expected, err.Error())
		}
	})

	t.Run("valid", func(t *testing.T) {
		date := scalar.NewDate(time.Now())
		if err := date.UnmarshalGQLContext(ctx, "2020-04-20"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if expected := "2020-04-20"; date.String() != expected {
			t.Errorf("expected date %q, got %q", expected, date.String())
		}
	})
}

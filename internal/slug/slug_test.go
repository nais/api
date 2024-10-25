package slug_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/nais/api/internal/slug"
)

func TestMarshalSlug(t *testing.T) {
	buf := new(bytes.Buffer)
	s := slug.Slug("some-slug")
	_ = s.MarshalGQLContext(context.Background(), buf)

	if expected := `"some-slug"`; buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestUnmarshalSlug(t *testing.T) {
	ctx := context.Background()
	t.Run("invalid case", func(t *testing.T) {
		s := slug.Slug("")
		err := s.UnmarshalGQLContext(ctx, 123)
		if expected := "slug must be a string"; err.Error() != expected {
			t.Errorf("expected error message %q, got %q", expected, err.Error())
		}
	})

	t.Run("valid case", func(t *testing.T) {
		s := slug.Slug("")
		if err := s.UnmarshalGQLContext(ctx, "slug"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if expected := "slug"; string(s) != expected {
			t.Errorf("expected %q, got %q", expected, string(s))
		}
	})
}

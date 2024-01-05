package slug_test

import (
	"bytes"
	"testing"

	"github.com/nais/api/internal/slug"
	"github.com/stretchr/testify/assert"
)

func TestMarshalSlug(t *testing.T) {
	buf := new(bytes.Buffer)
	s := slug.Slug("some-slug")
	s.MarshalGQL(buf)
	assert.Equal(t, `"some-slug"`, buf.String())
}

func TestUnmarshalSlug(t *testing.T) {
	t.Run("invalid case", func(t *testing.T) {
		s := slug.Slug("")
		err := s.UnmarshalGQL(123)
		assert.Equal(t, slug.Slug(""), s)
		assert.EqualError(t, err, "slug must be a string")
	})

	t.Run("valid case", func(t *testing.T) {
		s := slug.Slug("")
		err := s.UnmarshalGQL("slug")
		assert.NoError(t, err)
		assert.Equal(t, "slug", string(s))
	})
}

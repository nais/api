package scalar_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/nais/api/internal/graph/scalar"
	"github.com/stretchr/testify/assert"
)

func TestIdent_MarshalGQLContext(t *testing.T) {
	ctx := context.Background()

	t.Run("missing ID", func(t *testing.T) {
		buf := new(bytes.Buffer)
		ident := scalar.Ident{
			Type: scalar.IdentTypeTeam,
		}
		err := ident.MarshalGQLContext(ctx, buf)
		assert.EqualError(t, err, "id and type must be set")
	})

	t.Run("missing type", func(t *testing.T) {
		buf := new(bytes.Buffer)
		ident := scalar.Ident{
			ID: "some-id",
		}
		err := ident.MarshalGQLContext(ctx, buf)
		assert.EqualError(t, err, "id and type must be set")
	})

	t.Run("valid", func(t *testing.T) {
		buf := new(bytes.Buffer)
		ident := scalar.Ident{
			ID:   "some-id",
			Type: scalar.IdentTypeTeam,
		}
		err := ident.MarshalGQLContext(ctx, buf)
		assert.NoError(t, err)
		assert.Equal(t, `"aWQ9c29tZS1pZCZ0eXBlPXRlYW0="`, buf.String())
	})
}

func TestIdent_UnmarshalGQLContext(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid type", func(t *testing.T) {
		ident := scalar.Ident{}
		err := ident.UnmarshalGQLContext(ctx, 123)
		assert.EqualError(t, err, "ident must be a string")
	})

	t.Run("not base64", func(t *testing.T) {
		ident := scalar.Ident{}
		err := ident.UnmarshalGQLContext(ctx, "foobar")
		assert.ErrorContains(t, err, "illegal base64")
	})

	t.Run("valid", func(t *testing.T) {
		ident := scalar.Ident{}
		err := ident.UnmarshalGQLContext(ctx, "aWQ9c29tZS1pZCZ0eXBlPXRlYW0=")
		assert.NoError(t, err)
		assert.Equal(t, "some-id", ident.ID)
		assert.Equal(t, scalar.IdentTypeTeam, ident.Type)
	})
}

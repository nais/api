package directives_test

import (
	"context"
	"testing"

	"github.com/nais/api/internal/authz"
	"github.com/nais/api/internal/directives"
	"github.com/stretchr/testify/assert"
)

func TestAuth(t *testing.T) {
	var obj interface{}
	var nextHandler func(ctx context.Context) (res interface{}, err error)

	t.Run("No user in context", func(t *testing.T) {
		auth := directives.Auth()

		nextHandler = func(ctx context.Context) (res interface{}, err error) {
			panic("Should not be executed")
		}
		_, err := auth(context.Background(), obj, nextHandler)
		assert.ErrorIs(t, err, authz.ErrNotAuthenticated)
		assert.EqualError(t, err, "not authenticated")
	})
}

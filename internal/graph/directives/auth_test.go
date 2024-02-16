package directives_test

import (
	"context"
	"errors"
	"testing"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/directives"
)

func TestAuth(t *testing.T) {
	var obj interface{}
	var nextHandler func(ctx context.Context) (res interface{}, err error)

	t.Run("No user in context", func(t *testing.T) {
		auth := directives.Auth()

		nextHandler = func(ctx context.Context) (res interface{}, err error) {
			panic("Should not be executed")
		}
		if _, err := auth(context.Background(), obj, nextHandler); !errors.Is(err, authz.ErrNotAuthenticated) {
			t.Fatalf("expected error %v, got %v", authz.ErrNotAuthenticated, err)
		}
	})
}

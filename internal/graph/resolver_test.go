package graph

import (
	"context"
	"testing"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
)

func TestExecutableSchemaWithComplexity(t *testing.T) {
	schema := newExecutableSchemaWithComplexity(gengql.NewExecutableSchema(gengql.Config{}))
	ctx := context.Background()

	t.Run("non-paginated field returns false", func(t *testing.T) {
		if _, ok := schema.Complexity(ctx, "User", "name", 1, nil); ok {
			t.Fatal("expected non-paginated field to use default complexity")
		}
	})

	t.Run("paginated field defaults to DefaultPageSize", func(t *testing.T) {
		got, ok := schema.Complexity(ctx, "User", "teams", 4, nil)
		if !ok {
			t.Fatal("expected custom complexity for paginated field")
		}
		if want := pagination.DefaultPageSize * 4; got != want {
			t.Errorf("expected complexity %d, got %d", want, got)
		}
	})

	t.Run("paginated field uses first when provided", func(t *testing.T) {
		got, ok := schema.Complexity(ctx, "User", "teams", 3, map[string]any{"first": 10})
		if !ok {
			t.Fatal("expected custom complexity for paginated field")
		}
		if want := 30; got != want {
			t.Errorf("expected complexity %d, got %d", want, got)
		}
	})

	t.Run("paginated field uses last when first is nil", func(t *testing.T) {
		got, ok := schema.Complexity(ctx, "User", "teams", 5, map[string]any{"last": 7})
		if !ok {
			t.Fatal("expected custom complexity for paginated field")
		}
		if want := 35; got != want {
			t.Errorf("expected complexity %d, got %d", want, got)
		}
	})
}

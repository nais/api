package model_test

import (
	"testing"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestToOpenSearch(t *testing.T) {
	t.Run("missing namespace", func(t *testing.T) {
		_, err := model.ToOpenSearch(&unstructured.Unstructured{}, "env")
		if err == nil {
			t.Fatalf("expected error")
		}

		expected := "missing namespace"
		actual := err.Error()
		if actual != expected {
			t.Fatalf("expected %q, got %q", expected, actual)
		}
	})

	t.Run("missing instance name", func(t *testing.T) {
		u := &unstructured.Unstructured{}
		u.SetNamespace("team-name")
		_, err := model.ToOpenSearch(u, "env")
		if err == nil {
			t.Fatalf("expected error")
		}

		expected := "missing instance name"
		actual := err.Error()
		if actual != expected {
			t.Fatalf("expected %q, got %q", expected, actual)
		}
	})

	t.Run("happy path", func(t *testing.T) {
		const (
			envName        = "envName"
			teamName       = "team-name"
			openSearchName = "opensearch"
		)

		u := &unstructured.Unstructured{Object: map[string]interface{}{
			"status": map[string]interface{}{
				"state":      "Ready",
				"conditions": []interface{}{},
			},
		}}
		u.SetNamespace(teamName)
		u.SetName(openSearchName)

		m, err := model.ToOpenSearch(u, envName)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if expected := "envName-team-name-opensearch"; m.ID.ID != expected {
			t.Fatalf("expected %q, got %q", expected, m.ID.ID)
		}

		if m.ID.Type != scalar.IdentTypeOpenSearch {
			t.Fatalf("expected %q, got %q", scalar.IdentTypeOpenSearch, m.ID.Type)
		}

		if m.Name != openSearchName {
			t.Fatalf("expected %q, got %q", openSearchName, m.Name)
		}

		if m.Env.Name != envName {
			t.Fatalf("expected %q, got %q", envName, m.Env.Name)
		}

		if m.Env.Team != teamName {
			t.Fatalf("expected %q, got %q", teamName, m.Env.Team)
		}
	})
}

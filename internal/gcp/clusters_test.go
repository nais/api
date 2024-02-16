package gcp_test

import (
	"testing"

	"github.com/nais/api/internal/gcp"
)

func TestDecodeJSONToClusters(t *testing.T) {
	clusters := make(gcp.Clusters)

	t.Run("empty string", func(t *testing.T) {
		if err := clusters.Decode(""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(clusters) != 0 {
			t.Fatalf("expected empty clusters, got: %v", clusters)
		}
	})

	t.Run("empty JSON object", func(t *testing.T) {
		if err := clusters.Decode("{}"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(clusters) != 0 {
			t.Fatalf("expected empty clusters, got: %v", clusters)
		}
	})

	t.Run("JSON with clusters", func(t *testing.T) {
		if err := clusters.Decode(`{
			"env1": {"teams_folder_id": "123", "project_id": "some-id-123"},
			"env2": {"teams_folder_id": "456", "project_id": "some-id-456"}
		}`); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, exists := clusters["env1"]; !exists {
			t.Fatalf("expected cluster 'env1' to exist, got: %v", clusters)
		}

		if clusters["env1"].TeamsFolderID != 123 {
			t.Errorf("expected cluster 'env1' to have teams_folder_id 123, got: %v", clusters["env1"].TeamsFolderID)
		}

		if expected := "some-id-123"; clusters["env1"].ProjectID != expected {
			t.Errorf("expected cluster 'env1' to have project_id %q, got: %q", expected, clusters["env1"].ProjectID)
		}

		if _, exists := clusters["env2"]; !exists {
			t.Fatalf("expected cluster 'env2' to exist, got: %v", clusters)
		}

		if clusters["env2"].TeamsFolderID != 456 {
			t.Errorf("expected cluster 'env2' to have teams_folder_id 456, got: %v", clusters["env2"].TeamsFolderID)
		}

		if expected := "some-id-456"; clusters["env2"].ProjectID != expected {
			t.Errorf("expected cluster 'env2' to have project_id %q, got: %q", expected, clusters["env2"].ProjectID)
		}
	})
}

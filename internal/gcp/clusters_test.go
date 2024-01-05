package gcp_test

import (
	"testing"

	"github.com/nais/api/internal/gcp"
	"github.com/stretchr/testify/assert"
)

func TestDecodeJSONToClusters(t *testing.T) {
	clusters := make(gcp.Clusters)

	t.Run("empty string", func(t *testing.T) {
		err := clusters.Decode("")
		assert.NoError(t, err)
		assert.Len(t, clusters, 0)
	})

	t.Run("empty JSON object", func(t *testing.T) {
		err := clusters.Decode("{}")
		assert.NoError(t, err)
		assert.Len(t, clusters, 0)
	})

	t.Run("JSON with clusters", func(t *testing.T) {
		err := clusters.Decode(`{
			"env1": {"teams_folder_id": "123", "project_id": "some-id-123"},
			"env2": {"teams_folder_id": "456", "project_id": "some-id-456"}
		}`)
		assert.NoError(t, err)

		assert.Contains(t, clusters, "env1")
		assert.Equal(t, int64(123), clusters["env1"].TeamsFolderID)
		assert.Equal(t, "some-id-123", clusters["env1"].ProjectID)

		assert.Contains(t, clusters, "env2")
		assert.Equal(t, int64(456), clusters["env2"].TeamsFolderID)
		assert.Equal(t, "some-id-456", clusters["env2"].ProjectID)
	})
}

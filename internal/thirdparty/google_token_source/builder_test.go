package google_token_source_test

import (
	"testing"

	"github.com/nais/teams-backend/pkg/config"
	"github.com/nais/teams-backend/pkg/google_token_source"
	"github.com/stretchr/testify/assert"
)

func TestNewFromConfig(t *testing.T) {
	t.Run("Missing API_BACKEND_GOOGLE_MANAGEMENT_PROJECT_ID", func(t *testing.T) {
		cfg := &config.Config{}
		builder, err := google_token_source.NewFromConfig(cfg)
		assert.Nil(t, builder)
		assert.ErrorContains(t, err, "API_BACKEND_GOOGLE_MANAGEMENT_PROJECT_ID")
	})

	t.Run("Missing API_BACKEND_TENANT_DOMAIN", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.GoogleManagementProjectID = "some-project-id"
		builder, err := google_token_source.NewFromConfig(cfg)
		assert.Nil(t, builder)
		assert.ErrorContains(t, err, "API_BACKEND_TENANT_DOMAIN")
	})
}

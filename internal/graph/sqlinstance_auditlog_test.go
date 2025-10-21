package graph

import (
	"context"
	"fmt"
	"testing"

	"github.com/nais/api/internal/persistence/sqlinstance"
	"github.com/nais/api/internal/slug"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestContext(auditLogProjectID, auditLogLocation string) context.Context {
	ctx := context.Background()
	// Create minimal client for testing - we only need the context, not actual database operations
	client := &sqlinstance.Client{}
	return sqlinstance.NewLoaderContext(ctx, client, nil, nil, auditLogProjectID, auditLogLocation)
}

func TestSQLInstanceResolver_AuditLog(t *testing.T) {
	resolver := &sqlInstanceResolver{}

	t.Run("returns nil when pgaudit flag is not enabled", func(t *testing.T) {
		ctx := createTestContext("test-project", "test-location")
		instance := &sqlinstance.SQLInstance{
			Name:      "test-instance",
			ProjectID: "test-project",
			TeamSlug:  slug.Slug("test-team"),
			Flags: []*sqlinstance.SQLInstanceFlag{
				{Name: "some-other-flag", Value: "on"},
			},
		}

		result, err := resolver.AuditLog(ctx, instance)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns error when NAIS_AUDIT_LOG_PROJECT_ID is not set", func(t *testing.T) {
		ctx := createTestContext("", "europe-north1") // Empty project ID
		instance := &sqlinstance.SQLInstance{
			Name:      "test-instance",
			ProjectID: "test-project",
			TeamSlug:  slug.Slug("test-team"),
			Flags: []*sqlinstance.SQLInstanceFlag{
				{Name: "cloudsql.enable_pgaudit", Value: "on"},
			},
		}

		result, err := resolver.AuditLog(ctx, instance)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "NAIS_AUDIT_LOG_PROJECT_ID environment variable is required")
	})

	t.Run("returns error when NAIS_AUDIT_LOG_LOCATION is not set", func(t *testing.T) {
		ctx := createTestContext("nais-audit-logs-7178", "") // Empty location
		instance := &sqlinstance.SQLInstance{
			Name:      "test-instance",
			ProjectID: "test-project",
			TeamSlug:  slug.Slug("test-team"),
			Flags: []*sqlinstance.SQLInstanceFlag{
				{Name: "cloudsql.enable_pgaudit", Value: "on"},
			},
		}

		result, err := resolver.AuditLog(ctx, instance)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "NAIS_AUDIT_LOG_LOCATION environment variable is required")
	})

	t.Run("returns nil when pgaudit flag is set to off", func(t *testing.T) {
		ctx := createTestContext("nais-audit-logs-7178", "europe-north1")
		instance := &sqlinstance.SQLInstance{
			Name:      "test-instance",
			ProjectID: "test-project",
			TeamSlug:  slug.Slug("test-team"),
			Flags: []*sqlinstance.SQLInstanceFlag{
				{Name: "cloudsql.enable_pgaudit", Value: "off"},
			},
		}

		result, err := resolver.AuditLog(ctx, instance)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns AuditLog when pgaudit flag is enabled with various truthy values", func(t *testing.T) {
		truthyValues := []string{"on", "true", "1", "yes", "enabled", "ON", "TRUE", "YES", "ENABLED"}

		for _, value := range truthyValues {
			t.Run(fmt.Sprintf("value=%s", value), func(t *testing.T) {
				ctx := createTestContext("nais-audit-logs-7178", "europe-north1")
				instance := &sqlinstance.SQLInstance{
					Name:      "test-instance",
					ProjectID: "test-project",
					TeamSlug:  slug.Slug("test-team"),
					Flags: []*sqlinstance.SQLInstanceFlag{
						{Name: "cloudsql.enable_pgaudit", Value: value},
					},
				}

				result, err := resolver.AuditLog(ctx, instance)
				require.NoError(t, err)
				require.NotNil(t, result)

				// Verify the URL contains expected components
				assert.Contains(t, result.LogURL, "console.cloud.google.com/logs/query")
				assert.Contains(t, result.LogURL, "test-project%3Atest-instance") // URL encoded database_id
				assert.Contains(t, result.LogURL, "test-team")                    // team slug in storage scope
				assert.Contains(t, result.LogURL, "project=test-project")         // project parameter
			})
		}
	})

	t.Run("returns nil for falsy values", func(t *testing.T) {
		falsyValues := []string{"off", "false", "0", "no", "disabled", "OFF", "FALSE", "NO", "DISABLED", ""}

		for _, value := range falsyValues {
			t.Run(fmt.Sprintf("value=%s", value), func(t *testing.T) {
				ctx := createTestContext("nais-audit-logs-7178", "europe-north1")
				instance := &sqlinstance.SQLInstance{
					Name:      "test-instance",
					ProjectID: "test-project",
					TeamSlug:  slug.Slug("test-team"),
					Flags: []*sqlinstance.SQLInstanceFlag{
						{Name: "cloudsql.enable_pgaudit", Value: value},
					},
				}

				result, err := resolver.AuditLog(ctx, instance)
				require.NoError(t, err)
				assert.Nil(t, result)
			})
		}
	})

	t.Run("returns AuditLog when pgaudit flag is enabled", func(t *testing.T) {
		ctx := createTestContext("nais-audit-logs-7178", "europe-north1")
		instance := &sqlinstance.SQLInstance{
			Name:      "test-instance",
			ProjectID: "test-project",
			TeamSlug:  slug.Slug("test-team"),
			Flags: []*sqlinstance.SQLInstanceFlag{
				{Name: "cloudsql.enable_pgaudit", Value: "on"},
			},
		}

		result, err := resolver.AuditLog(ctx, instance)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify the URL contains expected components
		assert.Contains(t, result.LogURL, "console.cloud.google.com/logs/query")
		assert.Contains(t, result.LogURL, "test-project%3Atest-instance") // URL encoded database_id
		assert.Contains(t, result.LogURL, "test-team")                    // team slug in storage scope
		assert.Contains(t, result.LogURL, "project=test-project")         // project parameter

		// Verify the URL is reasonably long and contains the expected configuration
		assert.Greater(t, len(result.LogURL), 100) // URL should be reasonably long
		assert.Contains(t, result.LogURL, "projects%2Fnais-audit-logs-7178%2Flocations%2Feurope-north1")
	})

	t.Run("uses custom audit log configuration from context", func(t *testing.T) {
		ctx := createTestContext("custom-audit-project", "us-central1")
		instance := &sqlinstance.SQLInstance{
			Name:      "test-instance",
			ProjectID: "test-project",
			TeamSlug:  slug.Slug("test-team"),
			Flags: []*sqlinstance.SQLInstanceFlag{
				{Name: "cloudsql.enable_pgaudit", Value: "on"},
			},
		}

		result, err := resolver.AuditLog(ctx, instance)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify the URL uses custom configuration values
		assert.Contains(t, result.LogURL, "projects%2Fcustom-audit-project%2Flocations%2Fus-central1")
		assert.Contains(t, result.LogURL, "test-team") // team slug in storage scope
	})
}

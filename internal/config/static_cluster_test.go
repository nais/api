package config_test

import (
	"testing"

	"github.com/nais/api/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestStaticCluster_Decode(t *testing.T) {
	cluster := &config.StaticCluster{}
	t.Run("empty string", func(t *testing.T) {
		assert.NoError(t, cluster.EnvDecode(""))
	})

	t.Run("empty name", func(t *testing.T) {
		err := cluster.EnvDecode("|host|token")
		assert.ErrorContains(t, err, "Name must not be empty")
	})

	t.Run("empty host", func(t *testing.T) {
		err := cluster.EnvDecode("name||token")
		assert.ErrorContains(t, err, "Host must not be empty")
	})

	t.Run("empty token", func(t *testing.T) {
		err := cluster.EnvDecode("name|host|")
		assert.ErrorContains(t, err, "Token must not be empty")
	})

	t.Run("valid string", func(t *testing.T) {
		err := cluster.EnvDecode("name|host|token")
		assert.NoError(t, err)
		assert.Equal(t, "name", cluster.Name)
		assert.Equal(t, "host", cluster.Host)
		assert.Equal(t, "token", cluster.Token)
	})
}

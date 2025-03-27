package environmentmapper_test

import (
	"testing"

	"github.com/nais/api/internal/environmentmapper"
)

func TestMapper(t *testing.T) {
	environmentmapper.SetMapping(map[string]string{
		"dev":  "dev-gcp",
		"prod": "prod-gcp",
	})
	defer environmentmapper.SetMapping(nil)

	t.Run("Mapped", func(t *testing.T) {
		if expected, got := "dev-gcp", environmentmapper.EnvironmentName("dev"); got != expected {
			t.Errorf("Expected %q, got: %q", expected, got)
		}

		if expected, got := "prod-gcp", environmentmapper.EnvironmentName("prod"); got != expected {
			t.Errorf("Expected %q, got: %q", expected, got)
		}

		if expected, got := "foo", environmentmapper.EnvironmentName("foo"); got != expected {
			t.Errorf("Expected %q, got: %q", expected, got)
		}
	})

	t.Run("Unmapped", func(t *testing.T) {
		if expected, got := "dev", environmentmapper.ClusterName("dev-gcp"); got != expected {
			t.Errorf("Expected %q, got: %q", expected, got)
		}

		if expected, got := "prod", environmentmapper.ClusterName("prod-gcp"); got != expected {
			t.Errorf("Expected %q, got: %q", expected, got)
		}

		if expected, got := "foo", environmentmapper.ClusterName("foo"); got != expected {
			t.Errorf("Expected %q, got: %q", expected, got)
		}
	})
}

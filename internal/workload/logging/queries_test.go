package logging_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
	"github.com/nais/api/internal/workload/logging"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
)

func TestFromWorkload(t *testing.T) {
	environmentmapper.SetMapping(map[string]string{"dev": "dev-gcp"})
	t.Cleanup(func() { environmentmapper.SetMapping(nil) })

	configured := logging.EnvironmentDestinations{"dev": logging.SecureLogs, "prod": logging.Loki}
	for _, tt := range []struct {
		name        string
		environment string
		configured  logging.EnvironmentDestinations
		spec        *nais_io_v1.Logging
		want        []string
	}{
		{
			name: "mapped environment default", environment: "dev-gcp", configured: configured,
			want: []string{"secure_logs"},
		},
		{
			name: "different environment", environment: "prod", configured: configured,
			want: []string{"loki"},
		},
		{
			name: "enabled without explicit destinations", environment: "dev-gcp", configured: configured,
			spec: &nais_io_v1.Logging{Enabled: true},
			want: []string{"secure_logs"},
		},
		{
			name: "disabled logging", environment: "dev-gcp", configured: configured,
			spec: &nais_io_v1.Logging{Enabled: false},
			want: []string{},
		},
		{
			name: "explicit destinations", environment: "dev-gcp", configured: configured,
			spec: &nais_io_v1.Logging{
				Enabled: true,
				Destinations: []nais_io_v1.LogDestination{
					{ID: "loki"}, {ID: "secure_logs"}, {ID: "custom"},
				},
			},
			want: []string{"loki", "secure_logs", "custom"},
		},
		{
			name: "explicit secure logs overrides loki default", environment: "prod", configured: configured,
			spec: &nais_io_v1.Logging{
				Enabled:      true,
				Destinations: []nais_io_v1.LogDestination{{ID: "secure_logs"}},
			},
			want: []string{"secure_logs"},
		},
		{
			name: "unconfigured environment", environment: "unknown", configured: configured,
			want: []string{"loki"},
		},
		{
			name: "no configured destinations", environment: "unknown",
			want: []string{"loki"},
		},
		{
			name: "enabled fallback", environment: "unknown",
			spec: &nais_io_v1.Logging{Enabled: true},
			want: []string{"loki"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := logging.NewPackageContext(t.Context(), "test", tt.configured)
			base := workload.Base{
				Name:            "test-workload",
				TeamSlug:        "test-team",
				EnvironmentName: tt.environment,
				Logging:         tt.spec,
			}
			for _, wl := range []workload.Workload{
				&application.Application{Base: base},
				&job.Job{Base: base},
			} {
				got := make([]string, 0)
				for _, destination := range logging.FromWorkload(ctx, wl) {
					switch destination := destination.(type) {
					case logging.LogDestinationLoki:
						got = append(got, "loki")
					case logging.LogDestinationSecureLogs:
						got = append(got, "secure_logs")
					case logging.LogDestinationGeneric:
						got = append(got, destination.Name)
					default:
						t.Fatalf("unexpected destination type %T", destination)
					}
				}
				if diff := cmp.Diff(tt.want, got); diff != "" {
					t.Errorf("%T destinations mismatch (-want +got):\n%s", wl, diff)
				}
			}
		})
	}
}

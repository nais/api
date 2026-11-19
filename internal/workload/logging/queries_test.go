package logging_test

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
	"github.com/nais/api/internal/workload/logging"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
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

func TestFromWorkloadTeamLogs(t *testing.T) {
	environmentmapper.SetMapping(map[string]string{"dev-gcp": "dev"})
	t.Cleanup(func() { environmentmapper.SetMapping(nil) })

	for _, policyTest := range []struct {
		name        string
		teamSlug    slug.Slug
		environment string
		policy      *nais_io_v1.AccessPolicy
		rules       nais_io_v1.AccessPolicyRules
		match       bool
	}{
		{name: "no policy"},
		{name: "no outbound", policy: &nais_io_v1.AccessPolicy{}},
		{name: "empty outbound", policy: &nais_io_v1.AccessPolicy{Outbound: &nais_io_v1.AccessPolicyOutbound{}}},
		{
			name: "inbound only",
			policy: &nais_io_v1.AccessPolicy{Inbound: &nais_io_v1.AccessPolicyInbound{
				Rules: nais_io_v1.AccessPolicyInboundRules{{AccessPolicyRule: nais_io_v1.AccessPolicyRule{
					Application: "logging", Namespace: "nais-system",
				}}},
			}},
		},
		{
			name: "external only",
			policy: &nais_io_v1.AccessPolicy{Outbound: &nais_io_v1.AccessPolicyOutbound{
				External: []nais_io_v1.AccessPolicyExternalRule{{Host: "team-logs.nais-system"}},
			}},
		},
		{name: "documented rule", rules: nais_io_v1.AccessPolicyRules{{Application: "logging", Namespace: "nais-system"}}, match: true},
		{name: "local cluster", rules: nais_io_v1.AccessPolicyRules{{Application: "logging", Namespace: "nais-system", Cluster: "dev-gcp"}}, match: true},
		{
			name: "unmapped environment", environment: "dev-gcp",
			rules: nais_io_v1.AccessPolicyRules{{Application: "logging", Namespace: "nais-system", Cluster: "dev-gcp"}}, match: true,
		},
		{name: "other cluster", rules: nais_io_v1.AccessPolicyRules{{Application: "logging", Namespace: "nais-system", Cluster: "prod-gcp"}}},
		{name: "environment alias is not cluster", rules: nais_io_v1.AccessPolicyRules{{Application: "logging", Namespace: "nais-system", Cluster: "dev"}}},
		{name: "wrong application", rules: nais_io_v1.AccessPolicyRules{{Application: "team-logs", Namespace: "nais-system"}}},
		{name: "empty application", rules: nais_io_v1.AccessPolicyRules{{Namespace: "nais-system"}}},
		{name: "wrong namespace", rules: nais_io_v1.AccessPolicyRules{{Application: "logging", Namespace: "other-team"}}},
		{name: "default namespace is workload team", rules: nais_io_v1.AccessPolicyRules{{Application: "logging"}}},
		{
			name: "default namespace in nais-system", teamSlug: "nais-system",
			rules: nais_io_v1.AccessPolicyRules{{Application: "logging"}}, match: true,
		},
		{name: "wildcard application", rules: nais_io_v1.AccessPolicyRules{{Application: "*", Namespace: "nais-system"}}},
		{name: "wildcard namespace", rules: nais_io_v1.AccessPolicyRules{{Application: "logging", Namespace: "*"}}},
		{name: "wildcard cluster", rules: nais_io_v1.AccessPolicyRules{{Application: "logging", Namespace: "nais-system", Cluster: "*"}}},
		{name: "all wildcards", rules: nais_io_v1.AccessPolicyRules{{Application: "*", Namespace: "*", Cluster: "*"}}},
		{name: "partial wildcard", rules: nais_io_v1.AccessPolicyRules{{Application: "log*", Namespace: "nais-system"}}},
		{
			name: "multiple matching rules", match: true,
			rules: nais_io_v1.AccessPolicyRules{
				{Application: "other", Namespace: "nais-system"},
				{Application: "logging", Namespace: "nais-system", Cluster: "prod-gcp"},
				{Application: "logging", Namespace: "nais-system"},
				{Application: "logging", Namespace: "nais-system", Cluster: "dev-gcp"},
			},
		},
	} {
		t.Run(policyTest.name, func(t *testing.T) {
			policy := policyTest.policy
			if policyTest.rules != nil {
				policy = &nais_io_v1.AccessPolicy{Outbound: &nais_io_v1.AccessPolicyOutbound{Rules: policyTest.rules}}
			}
			for _, loggingTest := range []struct {
				name       string
				spec       *nais_io_v1.Logging
				configured logging.EnvironmentDestinations
				want       []string
			}{
				{name: "implicit loki", want: []string{"loki"}},
				{name: "enabled implicit loki", spec: &nais_io_v1.Logging{Enabled: true}, want: []string{"loki"}},
				{
					name: "environment loki", configured: logging.EnvironmentDestinations{"dev-gcp": logging.Loki},
					want: []string{"loki"},
				},
				{
					name: "environment secure logs", configured: logging.EnvironmentDestinations{"dev-gcp": logging.SecureLogs},
					want: []string{"secure_logs"},
				},
				{
					name: "explicit loki",
					spec: &nais_io_v1.Logging{Enabled: true, Destinations: slices.Grow([]nais_io_v1.LogDestination{{ID: "loki"}}, 1)},
					want: []string{"loki"},
				},
				{
					name: "explicit destinations",
					spec: &nais_io_v1.Logging{Enabled: true, Destinations: []nais_io_v1.LogDestination{{ID: "loki"}, {ID: "custom"}}},
					want: []string{"loki", "custom"},
				},
				{
					name: "explicit secure logs",
					spec: &nais_io_v1.Logging{Enabled: true, Destinations: []nais_io_v1.LogDestination{{ID: "loki"}, {ID: "secure_logs"}}},
					want: []string{"loki", "secure_logs"},
				},
				{name: "disabled logging", spec: &nais_io_v1.Logging{Enabled: false}, want: []string{}},
				{
					name: "disabled explicit destinations",
					spec: &nais_io_v1.Logging{Enabled: false, Destinations: []nais_io_v1.LogDestination{{ID: "loki"}, {ID: "secure_logs"}, {ID: "custom"}}},
					want: []string{},
				},
				{
					name: "disabled environment default",
					spec: &nais_io_v1.Logging{Enabled: false}, configured: logging.EnvironmentDestinations{"dev-gcp": logging.SecureLogs},
					want: []string{},
				},
			} {
				t.Run(loggingTest.name, func(t *testing.T) {
					spec := loggingTest.spec
					want := slices.Clone(loggingTest.want)
					if policyTest.match && !slices.Contains(want, "secure_logs") {
						want = append(want, "secure_logs")
					}
					base := workload.Base{
						Name: "test-workload", TeamSlug: "test-team", EnvironmentName: "dev",
						Logging: spec, AccessPolicy: policy,
					}
					if policyTest.teamSlug != "" {
						base.TeamSlug = policyTest.teamSlug
					}
					if policyTest.environment != "" {
						base.EnvironmentName = policyTest.environment
					}
					wantSpec := spec.DeepCopy()
					wantPolicy := policy.DeepCopy()
					var wantBacking []nais_io_v1.LogDestination
					if spec != nil {
						wantBacking = slices.Clone(spec.Destinations[:cap(spec.Destinations)])
					}
					applicationBase, jobBase := base, base
					applicationBase.Type = workload.TypeApplication
					jobBase.Type = workload.TypeJob
					for _, wl := range []workload.Workload{
						&application.Application{Base: applicationBase, Spec: &nais_io_v1alpha1.ApplicationSpec{
							AccessPolicy: policy, Observability: &nais_io_v1.Observability{Logging: spec},
						}},
						&job.Job{Base: jobBase, Spec: &nais_io_v1.NaisjobSpec{
							AccessPolicy: policy, Observability: &nais_io_v1.Observability{Logging: spec},
						}},
					} {
						t.Run(wl.GetType().String(), func(t *testing.T) {
							ctx := logging.NewPackageContext(t.Context(), "test", loggingTest.configured)
							first := logging.FromWorkload(ctx, wl)
							got := make([]string, 0, len(first))
							for _, destination := range first {
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
							if diff := cmp.Diff(want, got); diff != "" {
								t.Errorf("destinations mismatch (-want +got):\n%s", diff)
							}
							for range 3 {
								if next := logging.FromWorkload(ctx, wl); !slices.Equal(first, next) {
									t.Errorf("repeated call changed destinations: first %v, got %v", first, next)
								}
							}
							if diff := cmp.Diff(wantSpec, spec); diff != "" {
								t.Errorf("logging spec was mutated (-want +got):\n%s", diff)
							}
							if diff := cmp.Diff(wantPolicy, policy); diff != "" {
								t.Errorf("access policy was mutated (-want +got):\n%s", diff)
							}
							if spec != nil {
								if diff := cmp.Diff(wantBacking, spec.Destinations[:cap(spec.Destinations)]); diff != "" {
									t.Errorf("destination backing array was mutated (-want +got):\n%s", diff)
								}
							}
						})
					}
				})
			}
		})
	}
}

package kafkatopic_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/persistence/aivencredentials"
	"github.com/nais/api/internal/persistence/kafkatopic"
	"github.com/nais/api/internal/user"
	"github.com/sirupsen/logrus/hooks/test"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	dynfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestEnvironmentsValidation(t *testing.T) {
	environmentmapper.SetMapping(environmentmapper.EnvironmentMapping{"dev": "dev-gcp", "prod": "prod-gcp"})
	t.Cleanup(func() { environmentmapper.SetMapping(nil) })

	for _, tt := range []struct {
		name    string
		config  string
		wantErr string
	}{
		{name: "disabled", config: `{}`},
		{name: "canonical and alias", config: `{"dev-gcp":{"pool":"nav-dev","aliases":["dev"]}}`},
		{name: "shared pool", config: `{"dev-gcp":{"pool":"nav-dev"},"dev-fss":{"pool":"nav-dev"}}`},
		{name: "invalid JSON", config: `{`, wantErr: "unexpected end"},
		{name: "pool only", config: `{"dev-gcp":{"pool":"nav-dev"}}`},
		{name: "missing pool", config: `{"dev-gcp":{}}`, wantErr: "requires a name and pool"},
		{name: "unknown environment", config: `{"unknown":{"pool":"nav-dev"}}`, wantErr: "not a configured Nais environment"},
		{name: "cluster name is not canonical", config: `{"dev":{"pool":"nav-dev"}}`, wantErr: "not a configured Nais environment"},
		{name: "alias shadows configured canonical name", config: `{"dev-gcp":{"pool":"nav-dev","aliases":["prod-gcp"]},"prod-gcp":{"pool":"nav-prod"}}`, wantErr: "conflicts with a canonical environment"},
		{name: "alias shadows another Nais environment", config: `{"dev-gcp":{"pool":"nav-dev","aliases":["dev-fss"]}}`, wantErr: "conflicts with a Nais environment"},
		{name: "ambiguous alias", config: `{"dev-gcp":{"pool":"nav-dev","aliases":["legacy"]},"prod-gcp":{"pool":"nav-prod","aliases":["legacy"]}}`, wantErr: "configured for both"},
		{name: "empty alias", config: `{"dev-gcp":{"pool":"nav-dev","aliases":[""]}}`, wantErr: "empty alias"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var environments kafkatopic.Environments
			err := json.Unmarshal([]byte(tt.config), &environments)
			if err == nil {
				err = environments.Validate([]string{"dev", "prod", "dev-fss"})
			}
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("got %v, want error containing %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCreateKafkaCredentialsTargets(t *testing.T) {
	for _, tt := range []struct {
		name        string
		mapping     environmentmapper.EnvironmentMapping
		environment string
		targets     kafkatopic.Environments
		wantCluster string
		wantPool    string
		wantErr     string
	}{
		{
			name:        "Nav Nais environment",
			mapping:     environmentmapper.EnvironmentMapping{"dev": "dev-gcp"},
			environment: "dev-gcp",
			targets:     kafkatopic.Environments{"dev-gcp": {Pool: "nav-dev", Aliases: []string{"dev"}}},
			wantCluster: "dev", wantPool: "nav-dev",
		},
		{
			name:        "Nav legacy alias",
			mapping:     environmentmapper.EnvironmentMapping{"dev": "dev-gcp"},
			environment: "dev",
			targets:     kafkatopic.Environments{"dev-gcp": {Pool: "nav-dev", Aliases: []string{"dev"}}},
			wantCluster: "dev", wantPool: "nav-dev",
		},
		{
			name:        "production alias stays in production",
			mapping:     environmentmapper.EnvironmentMapping{"prod": "prod-gcp"},
			environment: "prod",
			targets:     kafkatopic.Environments{"prod-gcp": {Pool: "nav-prod", Aliases: []string{"prod"}}},
			wantCluster: "prod", wantPool: "nav-prod",
		},
		{
			name:        "another tenant with canonical dev",
			environment: "dev",
			targets:     kafkatopic.Environments{"dev": {Pool: "other-pool"}},
			wantCluster: "dev", wantPool: "other-pool",
		},
		{
			name:        "shared pool does not change the selected cluster",
			mapping:     environmentmapper.EnvironmentMapping{"dev": "dev-gcp"},
			environment: "dev-fss",
			targets: kafkatopic.Environments{
				"dev-gcp": {Pool: "nav-dev"},
				"dev-fss": {Pool: "nav-dev"},
			},
			wantCluster: "dev-fss", wantPool: "nav-dev",
		},
		{
			name:        "unknown environment is not guessed",
			environment: "prod-gcp",
			targets:     kafkatopic.Environments{"dev": {Pool: "some-pool"}},
			wantErr:     `not configured for Nais environment "prod-gcp"`,
		},
		{
			name:        "legacy cluster name requires explicit alias",
			mapping:     environmentmapper.EnvironmentMapping{"dev": "dev-gcp"},
			environment: "dev",
			targets:     kafkatopic.Environments{"dev-gcp": {Pool: "nav-dev"}},
			wantErr:     `not configured for Nais environment "dev"`,
		},
		{
			name:        "FSS does not fall back to GCP",
			mapping:     environmentmapper.EnvironmentMapping{"dev": "dev-gcp"},
			environment: "dev-fss",
			targets:     kafkatopic.Environments{"dev-gcp": {Pool: "nav-dev"}},
			wantErr:     `not configured for Nais environment "dev-fss"`,
		},
		{
			name:        "Kafka unavailable",
			environment: "dev",
			wantErr:     "not configured for any Nais environment",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			environmentmapper.SetMapping(tt.mapping)
			t.Cleanup(func() { environmentmapper.SetMapping(nil) })
			clusters := []string{"dev", "prod", "dev-fss"}
			if err := tt.targets.Validate(clusters); err != nil {
				t.Fatal(err)
			}

			// Stop at the Kubernetes write, before polling or audit logging, while
			// exercising the real credential request and client selection.
			stop := errors.New("stop at create")
			var gotCluster, gotPool, gotNamespace string
			clients := make(map[string]dynamic.Interface)
			for _, cluster := range clusters {
				client := dynfake.NewSimpleDynamicClient(runtime.NewScheme())
				client.PrependReactor("create", "aivenapplications", func(action k8stesting.Action) (bool, runtime.Object, error) {
					created := action.(k8stesting.CreateAction).GetObject().(*unstructured.Unstructured)
					var err error
					gotPool, _, err = unstructured.NestedString(created.Object, "spec", "kafka", "pool")
					if err != nil {
						t.Fatal(err)
					}
					gotCluster, gotNamespace = cluster, created.GetNamespace()
					return true, nil, stop
				})
				clients[cluster] = client
			}
			logger, _ := test.NewNullLogger()
			ctx := kafkatopic.NewLoaderContext(t.Context(), nil, logger, tt.targets)
			ctx = aivencredentials.NewClientContext(ctx, clients, logger)
			ctx = authz.ContextWithActor(ctx, &user.User{Email: "test@example.com"}, nil)
			_, err := kafkatopic.CreateKafkaCredentials(ctx, kafkatopic.CreateKafkaCredentialsInput{
				TeamSlug: "my-team", EnvironmentName: tt.environment, TTL: "1d",
			})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("got %v, want error containing %q", err, tt.wantErr)
				}
				for cluster, client := range clients {
					if actions := client.(*dynfake.FakeDynamicClient).Actions(); len(actions) > 0 {
						t.Errorf("unexpected Kubernetes actions in %s: %v", cluster, actions)
					}
				}
				return
			}
			if !errors.Is(err, stop) {
				t.Fatalf("got %v, want request to reach Kubernetes create", err)
			}
			if gotCluster != tt.wantCluster || gotPool != tt.wantPool || gotNamespace != "my-team" {
				t.Errorf("created in %s/%s with pool %s, want %s/my-team with pool %s", gotCluster, gotNamespace, gotPool, tt.wantCluster, tt.wantPool)
			}
		})
	}
}

package api

import (
	"bytes"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/persistence/kafkatopic"
	"gopkg.in/yaml.v3"
)

func TestKafkaEnvironmentFeatureDefaults(t *testing.T) {
	data, err := os.ReadFile("../../../charts/nais-api/Feature.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var feature struct {
		Values map[string]struct {
			Computed struct {
				Template string `yaml:"template"`
			} `yaml:"computed"`
		} `yaml:"values"`
	}
	if err := yaml.Unmarshal(data, &feature); err != nil {
		t.Fatal(err)
	}
	funcs := sprig.TxtFuncMap()
	funcs["toJSON"] = funcs["toJson"]
	tmpl, err := template.New("kafka environments").Funcs(funcs).Parse(feature.Values["kafka.environments"].Computed.Template)
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name   string
		tenant string
		envs   []map[string]string
		want   kafkatopic.Environments
	}{
		{
			name:   "tenant uses configured projects without naming assumptions",
			tenant: "example",
			envs: []map[string]string{
				{"kind": "tenant", "name": "dev", "aiven_project": "custom-project", "kafka_bootstrap_broker": "broker:9092"},
				{"kind": "tenant", "name": "non-prod", "aiven_project": "other-project", "kafka_bootstrap_broker": "other-broker:9092"},
			},
			want: kafkatopic.Environments{
				"dev":      {Pool: "custom-project"},
				"non-prod": {Pool: "other-project"},
			},
		},
		{
			name:   "environments without Kafka or a project and non-tenant environments are excluded",
			tenant: "example",
			envs: []map[string]string{
				{"kind": "tenant", "name": "dev", "aiven_project": "project-without-kafka"},
				{"kind": "tenant", "name": "prod", "kafka_bootstrap_broker": "broker:9092"},
				{"kind": "management", "name": "management", "aiven_project": "management", "kafka_bootstrap_broker": "broker:9092"},
				{"kind": "onprem", "name": "dev-fss", "aiven_project": "shared-project", "kafka_bootstrap_broker": "broker:9092"},
			},
			want: kafkatopic.Environments{},
		},
		{
			name:   "Nav retains its explicit topology and aliases",
			tenant: "nav",
			envs: []map[string]string{
				{"kind": "tenant", "name": "dev", "aiven_project": "different-project", "kafka_bootstrap_broker": "broker:9092"},
			},
			want: kafkatopic.Environments{
				"dev-gcp":  {Pool: "nav-dev", Aliases: []string{"dev"}},
				"prod-gcp": {Pool: "nav-prod", Aliases: []string{"prod"}},
			},
		},
		{name: "tenant without environments", tenant: "example", want: kafkatopic.Environments{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var rendered bytes.Buffer
			if err := tmpl.Execute(&rendered, map[string]any{
				"Tenant": map[string]string{"Name": tt.tenant},
				"Envs":   tt.envs,
			}); err != nil {
				t.Fatal(err)
			}
			value, err := strconv.Unquote(rendered.String())
			if err != nil {
				t.Fatalf("expected a quoted JSON Helm value: %v", err)
			}
			var got kafkatopic.Environments
			if err := json.Unmarshal([]byte(value), &got); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Kafka environment defaults mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

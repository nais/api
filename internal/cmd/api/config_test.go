package api

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/persistence/kafkatopic"
	"github.com/nais/api/internal/workload/logging"
	"github.com/sethvargo/go-envconfig"
)

func TestKafkaEnvironmentsConfig(t *testing.T) {
	for _, tt := range []struct {
		name    string
		value   string
		want    kafkatopic.Environments
		wantErr bool
	}{
		{
			name:  "configured",
			value: `{"dev-gcp":{"pool":"nav-dev","aliases":["dev"]}}`,
			want: kafkatopic.Environments{
				"dev-gcp": {Pool: "nav-dev", Aliases: []string{"dev"}},
			},
		},
		{name: "unset"},
		{name: "empty map", value: `{}`, want: kafkatopic.Environments{}},
		{name: "null", value: `null`},
		{name: "invalid JSON", value: `{`, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := NewConfig(t.Context(), envconfig.MapLookuper(map[string]string{
				"KAFKA_ENVIRONMENTS": tt.value,
			}))
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if diff := cmp.Diff(tt.want, cfg.KafkaEnvironments); diff != "" {
				t.Errorf("Kafka environment config mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestInsecureWebhookURLsConfig(t *testing.T) {
	for _, tt := range []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"secure by default", map[string]string{}, false},
		{"explicitly disabled", map[string]string{"WITH_INSECURE_WEBHOOK_URLS": "false"}, false},
		{"explicitly enabled", map[string]string{"WITH_INSECURE_WEBHOOK_URLS": "true"}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := NewConfig(t.Context(), envconfig.MapLookuper(tt.env))
			if err != nil {
				t.Fatal(err)
			}
			if got := cfg.Fakes.WithInsecureWebhookURLs; got != tt.want {
				t.Errorf("WithInsecureWebhookURLs = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoggingDestinationsConfig(t *testing.T) {
	for _, tt := range []struct {
		name    string
		value   string
		want    logging.EnvironmentDestinations
		wantErr bool
	}{
		{
			name:  "environment destinations",
			value: `{"dev":"loki","prod":"secure_logs"}`,
			want:  logging.EnvironmentDestinations{"dev": logging.Loki, "prod": logging.SecureLogs},
		},
		{name: "unset"},
		{name: "empty map", value: `{}`, want: logging.EnvironmentDestinations{}},
		{name: "null", value: `null`},
		{name: "invalid JSON", value: `{`, wantErr: true},
		{name: "array destination", value: `{"dev":["loki"]}`, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := NewConfig(t.Context(), envconfig.MapLookuper(map[string]string{
				"LOGGING_DESTINATIONS": tt.value,
			}))
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if diff := cmp.Diff(tt.want, cfg.Logging.Destinations); diff != "" {
				t.Errorf("logging destinations mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

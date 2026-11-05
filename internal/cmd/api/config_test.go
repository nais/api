package api

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/workload/logging"
	"github.com/sethvargo/go-envconfig"
)

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

func TestPostgresBranchWriteFreezeConfig(t *testing.T) {
	for _, tt := range []struct {
		name    string
		values  map[string]string
		want    bool
		wantErr bool
	}{
		{name: "enabled by default"},
		{name: "explicitly enabled", values: map[string]string{"POSTGRES_BRANCH_WRITES_DISABLED": "false"}},
		{name: "disabled for rollout", values: map[string]string{"POSTGRES_BRANCH_WRITES_DISABLED": "true"}, want: true},
		{name: "invalid boolean", values: map[string]string{"POSTGRES_BRANCH_WRITES_DISABLED": "invalid"}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := NewConfig(t.Context(), envconfig.MapLookuper(tt.values))
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewConfig() error = %v, wantErr %t", err, tt.wantErr)
			}
			if !tt.wantErr && cfg.PostgresBranchWritesDisabled != tt.want {
				t.Errorf("branch writes disabled = %t, want %t", cfg.PostgresBranchWritesDisabled, tt.want)
			}
		})
	}
}

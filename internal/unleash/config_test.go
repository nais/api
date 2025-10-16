package unleash

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nais/api/internal/environmentmapper"
	bifrost "github.com/nais/bifrost/pkg/unleash"
	"github.com/sirupsen/logrus/hooks/test"
)

// TestAllowedClustersMapping tests that clusters are correctly mapped through environmentmapper
// before being sent to Bifrost API
func TestAllowedClustersMapping(t *testing.T) {
	tests := []struct {
		name            string
		clusters        []string
		envMapping      environmentmapper.EnvironmentMapping
		expectedAllowed string
	}{
		{
			name:            "regular clusters without mapping",
			clusters:        []string{"dev-gcp", "prod-gcp", "dev-fss", "prod-fss"},
			envMapping:      nil,
			expectedAllowed: "dev-gcp,prod-gcp,dev-fss,prod-fss",
		},
		{
			name:     "clusters with environment mapping",
			clusters: []string{"dev", "prod"},
			envMapping: environmentmapper.EnvironmentMapping{
				"dev":  "dev-gcp",
				"prod": "prod-gcp",
			},
			expectedAllowed: "dev-gcp,prod-gcp",
		},
		{
			name:            "mixed clusters with partial mapping",
			clusters:        []string{"dev-gcp", "prod", "staging"},
			envMapping:      environmentmapper.EnvironmentMapping{"prod": "prod-gcp"},
			expectedAllowed: "dev-gcp,prod-gcp,staging",
		},
		{
			name:            "single cluster",
			clusters:        []string{"dev-gcp"},
			envMapping:      nil,
			expectedAllowed: "dev-gcp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment mapping
			if tt.envMapping != nil {
				environmentmapper.SetMapping(tt.envMapping)
			} else {
				environmentmapper.SetMapping(environmentmapper.EnvironmentMapping{})
			}
			defer environmentmapper.SetMapping(environmentmapper.EnvironmentMapping{})

			// Track the actual request made to bifrost
			var receivedConfig bifrost.UnleashConfig
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/unleash/new" {
					t.Errorf("expected /unleash/new, got %s", r.URL.Path)
					http.Error(w, "wrong path", http.StatusBadRequest)
					return
				}

				err := json.NewDecoder(r.Body).Decode(&receivedConfig)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				// Return a minimal valid response
				w.Write([]byte(`{}`))
			}))
			defer s.Close()

			// Create logger
			logger, _ := test.NewNullLogger()

			// Create a bifrost client
			client := NewBifrostClient(s.URL, logger)

			// Simulate what happens in Create function
			mappedClusters := make([]string, len(tt.clusters))
			for i, cluster := range tt.clusters {
				mappedClusters[i] = environmentmapper.EnvironmentName(cluster)
			}

			// Create the bifrost config
			bi := bifrost.UnleashConfig{
				Name:             "test-team",
				AllowedTeams:     "test-team",
				EnableFederation: true,
				AllowedClusters:  strings.Join(mappedClusters, ","),
			}

			// Make the request
			_, err := client.Post(context.Background(), "/unleash/new", bi)
			if err != nil {
				t.Fatalf("Post failed: %v", err)
			}

			// Verify the received config has correct allowed clusters
			if receivedConfig.AllowedClusters != tt.expectedAllowed {
				t.Errorf("AllowedClusters mismatch\nwant: %s\ngot:  %s",
					tt.expectedAllowed,
					receivedConfig.AllowedClusters)
			}

			// Verify basic config
			if receivedConfig.Name != "test-team" {
				t.Errorf("Name: want test-team, got %s", receivedConfig.Name)
			}
			if receivedConfig.AllowedTeams != "test-team" {
				t.Errorf("AllowedTeams: want test-team, got %s", receivedConfig.AllowedTeams)
			}
			if !receivedConfig.EnableFederation {
				t.Error("EnableFederation: want true, got false")
			}
		})
	}
}

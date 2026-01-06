package unleash

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/bifrost/pkg/bifrostclient"
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
			var receivedConfig bifrostclient.UnleashConfigRequest
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/unleash" {
					t.Errorf("expected /v1/unleash, got %s", r.URL.Path)
					http.Error(w, "wrong path", http.StatusBadRequest)
					return
				}
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
					http.Error(w, "wrong method", http.StatusBadRequest)
					return
				}

				err := json.NewDecoder(r.Body).Decode(&receivedConfig)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				// Return a minimal valid Unleash response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				name := receivedConfig.Name
				json.NewEncoder(w).Encode(bifrostclient.Unleash{
					Metadata: &struct {
						CreationTimestamp *time.Time `json:"creationTimestamp,omitempty"`
						Name              *string    `json:"name,omitempty"`
						Namespace         *string    `json:"namespace,omitempty"`
					}{
						Name: name,
					},
				})
			}))
			defer s.Close()

			// Create logger
			logger, _ := test.NewNullLogger()

			// Create a bifrost client
			client := NewBifrostClient(s.URL, logger)

			// Simulate what happens in newLoaders function
			mappedClusters := make([]string, len(tt.clusters))
			for i, cluster := range tt.clusters {
				mappedClusters[i] = environmentmapper.EnvironmentName(cluster)
			}
			allowedClustersStr := strings.Join(mappedClusters, ",")

			// Create the bifrost config request (as done in Create function)
			enableFederation := true
			teamName := "test-team"
			req := bifrostclient.UnleashConfigRequest{
				Name:             &teamName,
				AllowedTeams:     &teamName,
				EnableFederation: &enableFederation,
				AllowedClusters:  &allowedClustersStr,
			}

			// Make the request
			_, err := client.CreateInstance(context.Background(), req)
			if err != nil {
				t.Fatalf("CreateInstance failed: %v", err)
			}

			// Verify the received config has correct allowed clusters
			if receivedConfig.AllowedClusters == nil || *receivedConfig.AllowedClusters != tt.expectedAllowed {
				got := "<nil>"
				if receivedConfig.AllowedClusters != nil {
					got = *receivedConfig.AllowedClusters
				}
				t.Errorf("AllowedClusters mismatch\nwant: %s\ngot:  %s",
					tt.expectedAllowed,
					got)
			}

			// Verify basic config
			if receivedConfig.Name == nil || *receivedConfig.Name != "test-team" {
				got := "<nil>"
				if receivedConfig.Name != nil {
					got = *receivedConfig.Name
				}
				t.Errorf("Name: want test-team, got %s", got)
			}
			if receivedConfig.AllowedTeams == nil || *receivedConfig.AllowedTeams != "test-team" {
				got := "<nil>"
				if receivedConfig.AllowedTeams != nil {
					got = *receivedConfig.AllowedTeams
				}
				t.Errorf("AllowedTeams: want test-team, got %s", got)
			}
			if receivedConfig.EnableFederation == nil || !*receivedConfig.EnableFederation {
				t.Error("EnableFederation: want true, got false or nil")
			}
		})
	}
}

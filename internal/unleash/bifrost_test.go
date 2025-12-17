package unleash_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nais/api/internal/unleash"
	"github.com/nais/api/internal/unleash/bifrostclient"
	"github.com/sirupsen/logrus/hooks/test"
)

func TestBifrostClient_CreateInstance(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/unleash" {
			t.Error("expected /unleash, got", r.URL.Path)
		}

		var req bifrostclient.UnleashConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.Name == nil || *req.Name != "test" {
			t.Errorf("expected name 'test', got %v", req.Name)
		}
		if req.AllowedTeams == nil || *req.AllowedTeams != "team1,team2" {
			t.Errorf("expected allowed_teams 'team1,team2', got %v", req.AllowedTeams)
		}
		if req.EnableFederation == nil || !*req.EnableFederation {
			t.Error("expected enable_federation to be true")
		}
		if req.AllowedClusters == nil || *req.AllowedClusters != "cluster1,cluster2" {
			t.Errorf("expected allowed_clusters 'cluster1,cluster2', got %v", req.AllowedClusters)
		}

		// Return a response with the Bifrost Unleash structure
		now := time.Now()
		name := *req.Name
		namespace := "unleash"
		releaseChannel := "stable"
		version := "5.11.0"

		response := bifrostclient.Unleash{
			Metadata: &struct {
				CreationTimestamp *time.Time `json:"creationTimestamp,omitempty"`
				Name              *string    `json:"name,omitempty"`
				Namespace         *string    `json:"namespace,omitempty"`
			}{
				CreationTimestamp: &now,
				Name:              &name,
				Namespace:         &namespace,
			},
			Spec: &struct {
				CustomImage *string `json:"customImage,omitempty"`
				Federation  *struct {
					AllowedClusters *[]string `json:"allowedClusters,omitempty"`
					AllowedTeams    *[]string `json:"allowedTeams,omitempty"`
					Enabled         *bool     `json:"enabled,omitempty"`
				} `json:"federation,omitempty"`
				ReleaseChannel *string `json:"releaseChannel,omitempty"`
			}{
				ReleaseChannel: &releaseChannel,
			},
			Status: &struct {
				Connected *bool   `json:"connected,omitempty"`
				Version   *string `json:"version,omitempty"`
			}{
				Version: &version,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer s.Close()

	logger, _ := test.NewNullLogger()
	bifrostClient := unleash.NewBifrostClient(s.URL, logger)

	name := "test"
	allowedTeams := "team1,team2"
	enableFed := true
	allowedClusters := "cluster1,cluster2"
	req := bifrostclient.UnleashConfigRequest{
		Name:             &name,
		AllowedTeams:     &allowedTeams,
		EnableFederation: &enableFed,
		AllowedClusters:  &allowedClusters,
	}

	resp, err := bifrostClient.CreateInstance(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil || resp.JSON201 == nil {
		t.Fatal("expected response with JSON201, got nil")
	}
	if resp.JSON201.Metadata == nil || resp.JSON201.Metadata.Name == nil || *resp.JSON201.Metadata.Name != "test" {
		t.Errorf("expected name 'test', got %v", resp.JSON201.Metadata)
	}
}

func TestBifrostClient_UpdateInstance(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT method, got %s", r.Method)
		}
		if r.URL.Path != "/unleash/my-team" {
			t.Errorf("expected /unleash/my-team, got %s", r.URL.Path)
		}

		var req bifrostclient.UnleashConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.ReleaseChannelName == nil || *req.ReleaseChannelName != "rapid" {
			t.Errorf("expected release_channel_name 'rapid', got %v", req.ReleaseChannelName)
		}

		// Return updated instance
		name := "my-team"
		namespace := "unleash"
		releaseChannel := "rapid"
		version := "5.12.0-beta.1"

		response := bifrostclient.Unleash{
			Metadata: &struct {
				CreationTimestamp *time.Time `json:"creationTimestamp,omitempty"`
				Name              *string    `json:"name,omitempty"`
				Namespace         *string    `json:"namespace,omitempty"`
			}{
				Name:      &name,
				Namespace: &namespace,
			},
			Spec: &struct {
				CustomImage *string `json:"customImage,omitempty"`
				Federation  *struct {
					AllowedClusters *[]string `json:"allowedClusters,omitempty"`
					AllowedTeams    *[]string `json:"allowedTeams,omitempty"`
					Enabled         *bool     `json:"enabled,omitempty"`
				} `json:"federation,omitempty"`
				ReleaseChannel *string `json:"releaseChannel,omitempty"`
			}{
				ReleaseChannel: &releaseChannel,
			},
			Status: &struct {
				Connected *bool   `json:"connected,omitempty"`
				Version   *string `json:"version,omitempty"`
			}{
				Version: &version,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer s.Close()

	logger, _ := test.NewNullLogger()
	client := unleash.NewBifrostClient(s.URL, logger)

	releaseChannel := "rapid"
	req := bifrostclient.UnleashConfigRequest{
		ReleaseChannelName: &releaseChannel,
	}

	resp, err := client.UpdateInstance(context.Background(), "my-team", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil || resp.JSON200 == nil {
		t.Fatal("expected response with JSON200, got nil")
	}
	if resp.JSON200.Spec == nil || resp.JSON200.Spec.ReleaseChannel == nil || *resp.JSON200.Spec.ReleaseChannel != "rapid" {
		t.Errorf("expected release channel 'rapid', got %v", resp.JSON200.Spec)
	}
}

func TestBifrostClient_GetInstance(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET method, got %s", r.Method)
		}
		if r.URL.Path != "/unleash/my-team" {
			t.Errorf("expected /unleash/my-team, got %s", r.URL.Path)
		}

		name := "my-team"
		namespace := "unleash"
		version := "5.11.0"
		connected := true

		response := bifrostclient.Unleash{
			Metadata: &struct {
				CreationTimestamp *time.Time `json:"creationTimestamp,omitempty"`
				Name              *string    `json:"name,omitempty"`
				Namespace         *string    `json:"namespace,omitempty"`
			}{
				Name:      &name,
				Namespace: &namespace,
			},
			Status: &struct {
				Connected *bool   `json:"connected,omitempty"`
				Version   *string `json:"version,omitempty"`
			}{
				Version:   &version,
				Connected: &connected,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer s.Close()

	logger, _ := test.NewNullLogger()
	client := unleash.NewBifrostClient(s.URL, logger)

	resp, err := client.GetInstance(context.Background(), "my-team")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil || resp.JSON200 == nil {
		t.Fatal("expected response with JSON200, got nil")
	}
	if resp.JSON200.Metadata == nil || resp.JSON200.Metadata.Name == nil || *resp.JSON200.Metadata.Name != "my-team" {
		t.Errorf("expected name 'my-team', got %v", resp.JSON200.Metadata)
	}
}

func TestBifrostClient_DeleteInstance(t *testing.T) {
	deleted := false
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE method, got %s", r.Method)
		}
		if r.URL.Path != "/unleash/my-team" {
			t.Errorf("expected /unleash/my-team, got %s", r.URL.Path)
		}

		deleted = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer s.Close()

	logger, _ := test.NewNullLogger()
	client := unleash.NewBifrostClient(s.URL, logger)

	_, err := client.DeleteInstance(context.Background(), "my-team")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !deleted {
		t.Error("expected delete to be called")
	}
}

func TestBifrostClient_ListChannels(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET method, got %s", r.Method)
		}
		if r.URL.Path != "/releasechannels" {
			t.Errorf("expected /releasechannels, got %s", r.URL.Path)
		}

		stableType := "sequential"
		rapidType := "canary"
		stableTime := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
		rapidTime := time.Date(2024, 3, 20, 14, 15, 0, 0, time.UTC)

		channels := []bifrostclient.ReleaseChannelResponse{
			{
				Name:           "stable",
				CurrentVersion: "5.11.0",
				Type:           &stableType,
				LastUpdated:    &stableTime,
				CreatedAt:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Image:          "unleash:5.11.0",
			},
			{
				Name:           "rapid",
				CurrentVersion: "5.12.0-beta.1",
				Type:           &rapidType,
				LastUpdated:    &rapidTime,
				CreatedAt:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Image:          "unleash:5.12.0-beta.1",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(channels)
	}))
	defer s.Close()

	logger, _ := test.NewNullLogger()
	client := unleash.NewBifrostClient(s.URL, logger)

	resp, err := client.ListChannels(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil || resp.JSON200 == nil {
		t.Fatal("expected response with JSON200, got nil")
	}
	channels := *resp.JSON200
	if len(channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(channels))
	}

	if channels[0].Name != "stable" {
		t.Errorf("expected first channel name 'stable', got %s", channels[0].Name)
	}
	if channels[1].Name != "rapid" {
		t.Errorf("expected second channel name 'rapid', got %s", channels[1].Name)
	}
}

func TestBifrostClient_ErrorHandling_CreateInstance(t *testing.T) {
	tests := []struct {
		name            string
		statusCode      int
		response        string
		wantErrContains string
	}{
		{
			name:            "bad request with message",
			statusCode:      http.StatusBadRequest,
			response:        `{"error": "validation_error", "message": "Invalid input: release channel not found"}`,
			wantErrContains: "release channel not found",
		},
		{
			name:            "bad request for not found",
			statusCode:      http.StatusBadRequest,
			response:        `{"error": "not_found", "message": "Instance not found"}`,
			wantErrContains: "Instance not found",
		},
		{
			name:            "internal server error with message",
			statusCode:      http.StatusInternalServerError,
			response:        `{"error": "internal_error", "message": "Something went wrong"}`,
			wantErrContains: "Something went wrong",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.response))
			}))
			defer s.Close()

			logger, _ := test.NewNullLogger()
			client := unleash.NewBifrostClient(s.URL, logger)

			name := "test"
			_, err := client.CreateInstance(context.Background(), bifrostclient.UnleashConfigRequest{
				Name: &name,
			})

			if err == nil {
				t.Error("expected error but got nil")
				return
			}

			if !containsString(err.Error(), tt.wantErrContains) {
				t.Errorf("error message = %q, want to contain %q", err.Error(), tt.wantErrContains)
			}
		})
	}
}

func TestBifrostClient_ErrorHandling_UpdateInstance(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "not_found", "message": "Unleash instance does not exist"}`))
	}))
	defer s.Close()

	logger, _ := test.NewNullLogger()
	client := unleash.NewBifrostClient(s.URL, logger)

	releaseChannel := "stable"
	_, err := client.UpdateInstance(context.Background(), "my-team", bifrostclient.UnleashConfigRequest{
		ReleaseChannelName: &releaseChannel,
	})

	if err == nil {
		t.Error("expected error but got nil")
		return
	}

	if !containsString(err.Error(), "does not exist") {
		t.Errorf("error message = %q, want to contain 'does not exist'", err.Error())
	}
}

func TestBifrostClient_ErrorHandling_ListChannels(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "database_error", "message": "Failed to fetch release channels"}`))
	}))
	defer s.Close()

	logger, _ := test.NewNullLogger()
	client := unleash.NewBifrostClient(s.URL, logger)

	_, err := client.ListChannels(context.Background())

	if err == nil {
		t.Error("expected error but got nil")
		return
	}

	if !containsString(err.Error(), "Failed to fetch release channels") {
		t.Errorf("error message = %q, want to contain 'Failed to fetch release channels'", err.Error())
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

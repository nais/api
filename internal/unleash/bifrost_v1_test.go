package unleash_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/unleash"
	unleash_nais_io_v1 "github.com/nais/unleasherator/api/v1"
	"github.com/sirupsen/logrus/hooks/test"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBifrostClient_PostV1(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}

		if r.URL.Path != "/v1/unleash" {
			t.Errorf("expected /v1/unleash, got %s", r.URL.Path)
		}

		var req unleash.BifrostV1CreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		want := unleash.BifrostV1CreateRequest{
			Name:               "test-team",
			AllowedTeams:       "test-team",
			EnableFederation:   true,
			AllowedClusters:    "dev-gcp,prod-gcp",
			ReleaseChannelName: "stable",
		}
		if !cmp.Equal(want, req) {
			t.Errorf("request diff -want +got:\n%v", cmp.Diff(want, req))
		}

		unleashInstance := unleash_nais_io_v1.Unleash{
			ObjectMeta: v1.ObjectMeta{
				Name: req.Name,
			},
			Spec: unleash_nais_io_v1.UnleashSpec{
				ExtraEnvVars: []corev1.EnvVar{
					{Name: "TEAMS_ALLOWED_TEAMS", Value: req.AllowedTeams},
				},
				ReleaseChannel: unleash_nais_io_v1.UnleashReleaseChannelConfig{
					Name: req.ReleaseChannelName,
				},
			},
			Status: unleash_nais_io_v1.UnleashStatus{
				Version:    "5.11.0",
				Reconciled: true,
				Connected:  true,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(unleashInstance)
	}))
	defer s.Close()

	logger, _ := test.NewNullLogger()
	client := unleash.NewBifrostClient(s.URL, logger)

	req := unleash.BifrostV1CreateRequest{
		Name:               "test-team",
		AllowedTeams:       "test-team",
		EnableFederation:   true,
		AllowedClusters:    "dev-gcp,prod-gcp",
		ReleaseChannelName: "stable",
	}

	resp, err := client.Post(context.Background(), "/v1/unleash", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result unleash_nais_io_v1.Unleash
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Name != "test-team" {
		t.Errorf("expected name test-team, got %s", result.Name)
	}
	if result.Spec.ReleaseChannel.Name != "stable" {
		t.Errorf("expected release channel stable, got %s", result.Spec.ReleaseChannel.Name)
	}
}

func TestBifrostClient_PostV1WithCustomVersion(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req unleash.BifrostV1CreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.CustomVersion != "5.12.0" {
			t.Errorf("expected custom_version 5.12.0, got %s", req.CustomVersion)
		}
		if req.ReleaseChannelName != "" {
			t.Errorf("expected release_channel_name to be empty, got %s", req.ReleaseChannelName)
		}

		unleashInstance := unleash_nais_io_v1.Unleash{
			ObjectMeta: v1.ObjectMeta{Name: req.Name},
			Spec: unleash_nais_io_v1.UnleashSpec{
				CustomImage: "unleash-server:5.12.0",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(unleashInstance)
	}))
	defer s.Close()

	logger, _ := test.NewNullLogger()
	client := unleash.NewBifrostClient(s.URL, logger)

	req := unleash.BifrostV1CreateRequest{
		Name:             "test-team",
		AllowedTeams:     "test-team",
		EnableFederation: true,
		AllowedClusters:  "dev-gcp",
		CustomVersion:    "5.12.0",
	}

	resp, err := client.Post(context.Background(), "/v1/unleash", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
}

func TestBifrostClient_PutV1(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT method, got %s", r.Method)
		}

		if r.URL.Path != "/v1/unleash/my-team" {
			t.Errorf("expected /v1/unleash/my-team, got %s", r.URL.Path)
		}

		var req unleash.BifrostV1UpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		want := unleash.BifrostV1UpdateRequest{
			ReleaseChannelName: "rapid",
		}
		if !cmp.Equal(want, req) {
			t.Errorf("request diff -want +got:\n%v", cmp.Diff(want, req))
		}

		unleashInstance := unleash_nais_io_v1.Unleash{
			ObjectMeta: v1.ObjectMeta{Name: "my-team"},
			Spec: unleash_nais_io_v1.UnleashSpec{
				ReleaseChannel: unleash_nais_io_v1.UnleashReleaseChannelConfig{
					Name: req.ReleaseChannelName,
				},
			},
			Status: unleash_nais_io_v1.UnleashStatus{
				Version:    "5.12.0-beta.1",
				Reconciled: true,
				Connected:  true,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(unleashInstance)
	}))
	defer s.Close()

	logger, _ := test.NewNullLogger()
	client := unleash.NewBifrostClient(s.URL, logger)

	req := unleash.BifrostV1UpdateRequest{
		ReleaseChannelName: "rapid",
	}

	resp, err := client.Put(context.Background(), "/v1/unleash/my-team", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result unleash_nais_io_v1.Unleash
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Spec.ReleaseChannel.Name != "rapid" {
		t.Errorf("expected release channel rapid, got %s", result.Spec.ReleaseChannel.Name)
	}
}

func TestBifrostClient_PutV1WithAllowedTeams(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req unleash.BifrostV1UpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.AllowedTeams != "team1,team2,team3" {
			t.Errorf("expected allowed_teams team1,team2,team3, got %s", req.AllowedTeams)
		}

		unleashInstance := unleash_nais_io_v1.Unleash{
			ObjectMeta: v1.ObjectMeta{Name: "my-team"},
			Spec: unleash_nais_io_v1.UnleashSpec{
				ExtraEnvVars: []corev1.EnvVar{
					{Name: "TEAMS_ALLOWED_TEAMS", Value: req.AllowedTeams},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(unleashInstance)
	}))
	defer s.Close()

	logger, _ := test.NewNullLogger()
	client := unleash.NewBifrostClient(s.URL, logger)

	req := unleash.BifrostV1UpdateRequest{
		AllowedTeams: "team1,team2,team3",
	}

	resp, err := client.Put(context.Background(), "/v1/unleash/my-team", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
}

func TestBifrostClient_GetReleaseChannels(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET method, got %s", r.Method)
		}

		if r.URL.Path != "/v1/releasechannels" {
			t.Errorf("expected /v1/releasechannels, got %s", r.URL.Path)
		}

		channels := []unleash.BifrostV1ReleaseChannelResponse{
			{
				Name:           "stable",
				CurrentVersion: "5.11.0",
				Type:           "sequential",
				Description:    "Stable release channel",
				LastUpdated:    "2024-03-15T10:30:00Z",
			},
			{
				Name:           "rapid",
				CurrentVersion: "5.12.0-beta.1",
				Type:           "canary",
				Description:    "Rapid release channel",
				LastUpdated:    "2024-03-20T14:15:00Z",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(channels)
	}))
	defer s.Close()

	logger, _ := test.NewNullLogger()
	client := unleash.NewBifrostClient(s.URL, logger)

	resp, err := client.Get(context.Background(), "/v1/releasechannels")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	var channels []unleash.BifrostV1ReleaseChannelResponse
	if err := json.NewDecoder(resp.Body).Decode(&channels); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(channels))
	}

	if channels[0].Name != "stable" {
		t.Errorf("expected first channel name stable, got %s", channels[0].Name)
	}
	if channels[1].Name != "rapid" {
		t.Errorf("expected second channel name rapid, got %s", channels[1].Name)
	}
}

func TestBifrostClient_ErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		response   string
	}{
		{
			name:       "bad request",
			statusCode: http.StatusBadRequest,
			response:   `{"error": "validation_error", "message": "Invalid input"}`,
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			response:   `{"error": "not_found", "message": "Instance not found"}`,
		},
		{
			name:       "internal server error",
			statusCode: http.StatusInternalServerError,
			response:   `{"error": "internal_error", "message": "Something went wrong"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.response))
			}))
			defer s.Close()

			logger, _ := test.NewNullLogger()
			client := unleash.NewBifrostClient(s.URL, logger)

			_, err := client.Post(context.Background(), "/v1/unleash", unleash.BifrostV1CreateRequest{
				Name: "test",
			})

			if err == nil {
				t.Error("expected error but got nil")
			}
		})
	}
}

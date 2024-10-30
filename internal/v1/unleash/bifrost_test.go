package unleash_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/v1/unleash"
	bifrost "github.com/nais/bifrost/pkg/unleash"
	unleash_nais_io_v1 "github.com/nais/unleasherator/api/v1"
	"github.com/sirupsen/logrus/hooks/test"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBifrostClient_Post(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var u bifrost.UnleashConfig
		err := json.NewDecoder(r.Body).Decode(&u)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if r.URL.Path != "/unleash/new" {
			t.Error("expected /unleash/new, got", r.URL.Path)
		}

		want := bifrost.UnleashConfig{
			Name:             "test",
			AllowedTeams:     "team1,team2",
			EnableFederation: true,
			AllowedClusters:  "cluster1,cluster2",
		}
		if !cmp.Equal(want, u) {
			t.Errorf("diff -want +got:\n%v", cmp.Diff(want, u))
		}

		unleashInstance := unleash_nais_io_v1.Unleash{
			ObjectMeta: v1.ObjectMeta{
				Name: u.Name,
			},
			Spec: unleash_nais_io_v1.UnleashSpec{
				ExtraEnvVars: []corev1.EnvVar{
					{
						Name:  "TEAMS_ALLOWED_TEAMS",
						Value: u.AllowedTeams,
					},
				},
				Federation: unleash_nais_io_v1.UnleashFederationConfig{
					Enabled:     true,
					Clusters:    []string{"cluster1", "cluster2"},
					SecretNonce: "abc123",
					Namespaces:  []string{"team1", "team2"},
				},
			},
		}

		unleashJSON, err := json.Marshal(unleashInstance)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(unleashJSON)
	}))
	defer s.Close()

	logger, _ := test.NewNullLogger()
	bifrostClient := unleash.NewBifrostClient(s.URL, logger)
	cfg := &bifrost.UnleashConfig{
		Name:             "test",
		AllowedTeams:     "team1,team2",
		EnableFederation: true,
		AllowedClusters:  "cluster1,cluster2",
	}
	resp, err := bifrostClient.Post(context.Background(), "/unleash/new", cfg)
	if err != nil {
		t.Fatal(err)
	}

	var unleashInstance unleash_nais_io_v1.Unleash
	err = json.NewDecoder(resp.Body).Decode(&unleashInstance)
	if err != nil {
		t.Fatal(err)
	}
}

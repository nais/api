package unleash_test

import (
	"context"
	"encoding/json"
	"github.com/nais/api/internal/unleash"
	bifrost "github.com/nais/bifrost/pkg/unleash"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBifrostClient_NewUnleash(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var u bifrost.UnleashConfig
		err := json.NewDecoder(r.Body).Decode(&u)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		assert.Equal(t, "/unleash/new", r.URL.Path)
		assert.Equal(t, "test", u.Name)
		w.Write([]byte(`{"instance": "test"}`))
	}))
	defer s.Close()

	logger, _ := test.NewNullLogger()
	bifrostClient := unleash.NewBifrostClient(s.URL, logger)
	err := bifrostClient.NewUnleash(context.Background(), "test", []string{"team1", "team2"})
	assert.NoError(t, err)
}

func TestBifrostClient_UpdateUnleash(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var u bifrost.UnleashConfig
		err := json.NewDecoder(r.Body).Decode(&u)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		assert.Equal(t, "/unleash/edit", r.URL.Path)
		assert.Equal(t, "test", u.Name)
		w.Write([]byte(`{"instance": "test"}`))
	}))
	defer s.Close()

	logger, _ := test.NewNullLogger()
	bifrostClient := unleash.NewBifrostClient(s.URL, logger)
	err := bifrostClient.UpdateUnleash(context.Background(), "test", []string{"team1", "team2"})
	assert.NoError(t, err)
}

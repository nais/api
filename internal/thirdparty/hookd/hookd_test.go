package hookd_test

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	httptest "github.com/nais/api/internal/test"
	"github.com/nais/api/internal/thirdparty/hookd"
	"github.com/sirupsen/logrus/hooks/test"
)

func TestClient(t *testing.T) {
	ctx := context.Background()
	logger, _ := test.NewNullLogger()
	psk := "psk"

	t.Run("get deploykey errors when error is returned from backend", func(t *testing.T) {
		hookdServer := httptest.NewHttpServerWithHandlers(t, []http.HandlerFunc{
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		})

		endpoint := hookdServer.URL
		client := hookd.New(endpoint, psk, logger)

		deployKey, err := client.DeployKey(ctx, "team")
		if deployKey != nil {
			t.Fatalf("expected deployKey to be nil, got %v", deployKey)
		}
		if !strings.Contains(err.Error(), "Internal Server Error") {
			t.Fatalf("expected error to be %q, got %q", "Internal Server Error", err.Error())
		}
	})

	t.Run("get deploykey errors when response from server is invalid", func(t *testing.T) {
		hookdServer := httptest.NewHttpServerWithHandlers(t, []http.HandlerFunc{
			func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("some string"))
			},
		})

		endpoint := hookdServer.URL
		client := hookd.New(endpoint, psk, logger)

		_, err := client.DeployKey(ctx, "team")
		if !strings.Contains(err.Error(), "invalid reply from server:") {
			t.Fatalf("expected error to be %q, got %q", "invalid reply from server:", err.Error())
		}
	})

	t.Run("get deploykey", func(t *testing.T) {
		hookdServer := httptest.NewHttpServerWithHandlers(t, []http.HandlerFunc{
			func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"team":"some-team", "key":"some-key"}`))
			},
		})

		endpoint := hookdServer.URL
		client := hookd.New(endpoint, psk, logger)

		key, err := client.DeployKey(ctx, "team")
		if err != nil {
			t.Fatal(err)
		}
		want := &hookd.DeployKey{
			Team: "some-team",
			Key:  "some-key",
		}
		if diff := cmp.Diff(want, key); diff != "" {
			t.Errorf("diff: -want +got\n%s", diff)
		}
	})
}

func TestRequestOptions(t *testing.T) {
	const team = "team"
	const cluster = "cluster"
	const limit = 42
	ignoreTeams := []string{"team1", "team2"}

	r, err := http.NewRequest(http.MethodGet, "http://localhost", nil)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	hookd.WithTeam(team)(r)
	hookd.WithCluster(cluster)(r)
	hookd.WithLimit(limit)(r)
	hookd.WithIgnoreTeams(ignoreTeams...)(r)

	want := url.Values{
		"team":       []string{team},
		"cluster":    []string{cluster},
		"limit":      []string{strconv.Itoa(limit)},
		"ignoreTeam": []string{strings.Join(ignoreTeams, ",")},
	}
	if diff := cmp.Diff(want, r.URL.Query()); diff != "" {
		t.Errorf("diff: -want +got\n%s", diff)
	}
}

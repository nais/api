package unleash

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
)

// The pre-shared key must reach bifrost on every request. bifrost accepts
// "Authorization: Bearer <key>"; sending nothing is only tolerated until it
// enables enforcement, after which an unauthenticated call is a 401.
func TestNewBifrostClient_SendsPreSharedKey(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(srv.Close)

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	client := NewBifrostClient(srv.URL, "s3cret", logger)
	if _, err := client.ListInstances(context.Background()); err != nil {
		t.Fatalf("list: %v", err)
	}

	if want := "Bearer s3cret"; gotAuth != want {
		t.Fatalf("Authorization header = %q, want %q", gotAuth, want)
	}
}

// An unset key must not send an empty or malformed header — that would be worse
// than sending none, since bifrost would see a present-but-invalid credential.
func TestNewBifrostClient_NoKeySendsNoHeader(t *testing.T) {
	var hadAuth bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hadAuth = r.Header["Authorization"]
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(srv.Close)

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	client := NewBifrostClient(srv.URL, "", logger)
	if _, err := client.ListInstances(context.Background()); err != nil {
		t.Fatalf("list: %v", err)
	}

	if hadAuth {
		t.Fatal("no key configured, but an Authorization header was sent")
	}
}

// bifrost and nais-api read the same fasit value, and bifrost accepts a
// comma-separated list so keys can be rotated without downtime. A client must
// present exactly one of them — sending the list verbatim matches nothing, and
// under enforcement that is a full outage at the worst possible moment.
func TestActiveBifrostAPIKey(t *testing.T) {
	for _, tc := range []struct {
		name, configured, want string
	}{
		{"single key", "abc123", "abc123"},
		{"rotation: first is active", "new,old", "new"},
		{"whitespace is trimmed", " new , old ", "new"},
		{"empty stays empty", "", ""},
		{"only separators is empty", " , ", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ActiveBifrostAPIKey(tc.configured); got != tc.want {
				t.Fatalf("ActiveBifrostAPIKey(%q) = %q, want %q", tc.configured, got, tc.want)
			}
		})
	}
}

// The whole point of picking one key: a rotation value must produce a header
// bifrost can actually match.
func TestNewBifrostClient_SendsOneKeyDuringRotation(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(srv.Close)

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	client := NewBifrostClient(srv.URL, "newkey,oldkey", logger)
	if _, err := client.ListInstances(context.Background()); err != nil {
		t.Fatalf("list: %v", err)
	}

	if want := "Bearer newkey"; gotAuth != want {
		t.Fatalf("Authorization header = %q, want %q — the list must not be sent verbatim", gotAuth, want)
	}
}

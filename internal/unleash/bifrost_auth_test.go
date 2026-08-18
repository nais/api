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

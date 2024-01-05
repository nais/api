package test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// NewHttpServerWithHandlers creates a new httptest.Server with the provided handlers.
func NewHttpServerWithHandlers(t *testing.T, handlers []http.HandlerFunc) *httptest.Server {
	idx := 0
	t.Cleanup(func() {
		if diff := len(handlers) - idx; diff != 0 {
			t.Fatalf("too many configured handlers, remove %d handler(s)", diff)
		}
	})
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(handlers) < idx+1 {
			t.Fatalf("unexpected request, add missing handler func: %v", r)
		}
		handlers[idx](w, r)
		idx += 1
	}))
}

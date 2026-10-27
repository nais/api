package loki

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/go-cmp/cmp"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

func TestProxyHTTP(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		query       string
		body        string
		contentType string
	}{
		{
			name:   "query without environment enforcement",
			method: http.MethodGet,
			path:   "/loki/api/v1/query_range",
			query:  url.Values{"query": {`{k8s_cluster_name=~"dev|prod"}`, `{app="test"}`}, "limit": {"10"}}.Encode(),
		},
		{
			name:        "form body and query parameters are preserved",
			method:      http.MethodPost,
			path:        "/loki/api/v1/series",
			query:       url.Values{"match[]": {`{app="query"}`}}.Encode(),
			body:        url.Values{"match[]": {`{k8s_cluster_name="prod"}`, `{app="body"}`}}.Encode(),
			contentType: "application/x-www-form-urlencoded",
		},
		{
			name:        "request validation is left to Loki",
			method:      http.MethodPost,
			path:        "/loki/api/v1/query",
			query:       "query=not-logql",
			body:        `{"query":"unchanged"}`,
			contentType: "application/json",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method || r.URL.Path != tt.path || r.URL.RawQuery != tt.query {
					t.Errorf("unexpected upstream request: %s %s", r.Method, r.URL.String())
				}
				if got := r.Header.Values("X-Scope-OrgID"); !cmp.Equal(got, []string{"tenant"}) {
					t.Errorf("unexpected tenant header: %v", got)
				}
				for _, header := range []string{"Authorization", "Cookie", "X-User-Email", "X-Forwarded-Host"} {
					if r.Header.Get(header) != "" {
						t.Errorf("unexpected upstream header %s", header)
					}
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if string(body) != tt.body || r.Header.Get("Content-Type") != tt.contentType {
					t.Errorf("request body or content type changed: %q, %q", body, r.Header.Get("Content-Type"))
				}
				w.Header().Set("X-Loki-Test", "forwarded")
				w.WriteHeader(http.StatusPartialContent)
				_, _ = io.WriteString(w, `{"data":"unchanged"}`)
			}))
			defer upstream.Close()

			proxy := mountedProxy(t, upstream.URL)
			req := httptest.NewRequest(tt.method, "/api"+tt.path+"?"+tt.query, strings.NewReader(tt.body))
			req.Header.Set("Authorization", "Bearer test-token")
			req.Header.Set("Cookie", "session_id=test-session")
			req.Header.Set("X-User-Email", "user@example.com")
			req.Header.Add("X-Scope-OrgID", "other-tenant")
			req.Header.Add("X-Scope-OrgID", "another-tenant")
			req.Header.Set("X-Forwarded-Host", "untrusted.example")
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			resp := httptest.NewRecorder()
			proxy.ServeHTTP(resp, req)
			if resp.Code != http.StatusPartialContent || resp.Body.String() != `{"data":"unchanged"}` {
				t.Fatalf("unexpected response: %d %s", resp.Code, resp.Body.String())
			}
			if resp.Header().Get("X-Loki-Test") != "forwarded" {
				t.Error("upstream response header was not forwarded")
			}
		})
	}
}

func TestProxyRejectsCrossOriginWebSocket(t *testing.T) {
	proxy := mountedProxy(t, "http://loki-querier:3100")
	req := httptest.NewRequest("GET", "/api/loki/api/v1/tail", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Origin", "https://other.example.com")
	resp := httptest.NewRecorder()
	proxy.ServeHTTP(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Errorf("got %d, want 403: %s", resp.Code, resp.Body.String())
	}
}

func TestProxyWebSocket(t *testing.T) {
	query := url.Values{"query": {`{k8s_cluster_name=~"dev|prod"}`}}.Encode()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loki/api/v1/tail" || r.URL.RawQuery != query {
			t.Errorf("unexpected upstream URL: %s", r.URL.String())
		}
		if r.Header.Get("X-Scope-OrgID") != "tenant" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("incorrect upstream authentication headers")
		}
		upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Error(err)
			return
		}
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			t.Error(err)
			return
		}
		if err := conn.WriteMessage(messageType, message); err != nil {
			t.Error(err)
		}
	}))
	defer upstream.Close()

	server := httptest.NewServer(mountedProxy(t, upstream.URL))
	defer server.Close()
	for _, origin := range []string{"", server.URL} {
		t.Run("origin="+origin, func(t *testing.T) {
			target := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/loki/api/v1/tail?" + query
			headers := http.Header{"Authorization": {"Bearer test-token"}, "Cookie": {"session_id=test-session"}}
			if origin != "" {
				headers.Set("Origin", origin)
			}
			conn, resp, err := websocket.DefaultDialer.DialContext(t.Context(), target, headers)
			if err != nil {
				if resp != nil {
					defer resp.Body.Close()
				}
				t.Fatal(err)
			}
			defer conn.Close()
			if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"streams":[]}`)); err != nil {
				t.Fatal(err)
			}
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				t.Fatal(err)
			}
			if messageType != websocket.TextMessage || string(message) != `{"streams":[]}` {
				t.Errorf("unexpected WebSocket frame: %d %s", messageType, message)
			}
		})
	}
}

func TestProxyUpstreamFailure(t *testing.T) {
	upstream := httptest.NewServer(http.NotFoundHandler())
	upstream.Close()
	proxy := mountedProxy(t, upstream.URL)
	resp := httptest.NewRecorder()
	proxy.ServeHTTP(resp, httptest.NewRequest("GET", "/api/loki/api/v1/labels", nil))
	if resp.Code != http.StatusBadGateway {
		t.Errorf("got %d, want 502", resp.Code)
	}
}

func TestProxyInvalidAddress(t *testing.T) {
	for _, address := range []string{"", "ws://loki:3100", "http://", "http://user:pass@loki:3100", "http://loki:3100?query=override", "http://loki:3100/#fragment"} {
		t.Run(address, func(t *testing.T) {
			if _, err := NewProxy(address, logrus.New()); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func mountedProxy(t *testing.T, address string) http.Handler {
	t.Helper()
	proxy, err := NewProxy(address, logrus.New())
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	router.Mount("/api/loki", http.StripPrefix("/api", proxy))
	return router
}

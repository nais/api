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
	"github.com/nais/api/internal/environmentmapper"
	"github.com/sirupsen/logrus"
)

func TestScopeProxyParameters(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		params   url.Values
		want     url.Values
		wantErr  bool
	}{
		{
			name:     "log query and other parameters",
			endpoint: "query_range",
			params:   url.Values{"query": {`{app="test"} |= "error"`}, "limit": {"10"}},
			want:     url.Values{"query": {`{app="test", k8s_cluster_name="dev"} |= "error"`}, "limit": {"10"}},
		},
		{
			name:     "all selectors in a metric expression",
			endpoint: "query",
			params:   url.Values{"query": {`sum(rate({app="a"}[5m])) / sum(rate({app="b"}[5m]))`}},
			want:     url.Values{"query": {`(sum(rate({app="a", k8s_cluster_name="dev"}[5m])) / sum(rate({app="b", k8s_cluster_name="dev"}[5m])))`}},
		},
		{
			name:     "conflicting matcher cannot override scope",
			endpoint: "tail",
			params:   url.Values{"query": {`{k8s_cluster_name="prod"}`}},
			want:     url.Values{"query": {`{k8s_cluster_name="prod", k8s_cluster_name="dev"}`}},
		},
		{
			name:     "regex cannot override scope",
			endpoint: "query_range",
			params:   url.Values{"query": {`{k8s_cluster_name=~".+"}`}},
			want:     url.Values{"query": {`{k8s_cluster_name=~".+", k8s_cluster_name="dev"}`}},
		},
		{
			name:     "matching scope is not duplicated",
			endpoint: "tail",
			params:   url.Values{"query": {`{k8s_cluster_name="dev"}`}},
			want:     url.Values{"query": {`{k8s_cluster_name="dev"}`}},
		},
		{
			name:     "duplicate query parameters become one scoped query",
			endpoint: "query_range",
			params:   url.Values{"query": {`{app="a"}`, `{app="b"}`}},
			want:     url.Values{"query": {`{app="a", k8s_cluster_name="dev"}`}},
		},
		{
			name:     "labels without a selector",
			endpoint: "labels",
			params:   url.Values{},
			want:     url.Values{"query": {`{k8s_cluster_name="dev"}`}},
		},
		{
			name:     "label values with an empty selector",
			endpoint: "label/app/values",
			params:   url.Values{"query": {"{}"}},
			want:     url.Values{"query": {`{k8s_cluster_name="dev"}`}},
		},
		{
			name:     "index statistics without a selector",
			endpoint: "index/stats",
			params:   url.Values{},
			want:     url.Values{"query": {`{k8s_cluster_name="dev"}`}},
		},
		{
			name:     "series without selectors",
			endpoint: "series",
			params:   url.Values{},
			want:     url.Values{"match[]": {`{k8s_cluster_name="dev"}`}},
		},
		{
			name:     "series scopes both match aliases and every union member",
			endpoint: "series",
			params:   url.Values{"match[]": {`{app="a"}`, "{}"}, "match": {`{k8s_cluster_name!="dev"}`}},
			want:     url.Values{"match[]": {`{app="a", k8s_cluster_name="dev"}`, `{k8s_cluster_name="dev"}`, `{k8s_cluster_name!="dev", k8s_cluster_name="dev"}`}},
		},
		{
			name:     "invalid LogQL",
			endpoint: "query",
			params:   url.Values{"query": {"not LogQL"}},
			wantErr:  true,
		},
		{
			name:     "missing tail query",
			endpoint: "tail",
			params:   url.Values{},
			wantErr:  true,
		},
		{
			name:     "invalid series selector",
			endpoint: "series",
			params:   url.Values{"match[]": {`{app="a"} |= "error"`}},
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := scopeProxyParameters(tt.params, tt.endpoint, "dev")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, tt.params); diff != "" {
				t.Errorf("parameters mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestProxyHTTP(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loki/api/v1/query_range" {
			t.Errorf("unexpected upstream path: %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("query"); got != `{app="test", k8s_cluster_name="dev"}` {
			t.Errorf("unexpected query: %q", got)
		}
		if r.URL.Query().Get("limit") != "10" {
			t.Error("limit was not forwarded")
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
		if len(body) != 0 {
			t.Errorf("unscoped request body was forwarded: %q", body)
		}
		w.Header().Set("X-Loki-Test", "forwarded")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = io.WriteString(w, `{"data":"unchanged"}`)
	}))
	defer upstream.Close()

	environmentmapper.SetMapping(environmentmapper.EnvironmentMapping{"dev": "dev-gcp"})
	defer environmentmapper.SetMapping(nil)
	proxy := mountedProxy(t, upstream.URL)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			params := url.Values{"query": {`{app="test"}`}, "limit": {"10"}}
			target := "/api/loki/dev-gcp/loki/api/v1/query_range"
			var body io.Reader
			if method == http.MethodPost {
				body = strings.NewReader(params.Encode())
				target += "?query=" + url.QueryEscape(`{k8s_cluster_name="prod"}`)
			} else {
				target += "?" + params.Encode()
			}
			req := httptest.NewRequest(method, target, body)
			if method == http.MethodPost {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
			req.Header.Set("Authorization", "Bearer api-secret")
			req.Header.Set("Cookie", "session_id=secret")
			req.Header.Set("X-User-Email", "user@example.com")
			req.Header.Add("X-Scope-OrgID", "other-tenant")
			req.Header.Add("X-Scope-OrgID", "another-tenant")
			req.Header.Set("X-Forwarded-Host", "untrusted.example")
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

func TestProxyRejectsRequests(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("rejected request reached upstream")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()
	proxy := mountedProxy(t, upstream.URL)
	tests := []struct {
		name        string
		method      string
		path        string
		body        string
		contentType string
		upgrade     bool
		want        int
	}{
		{name: "unknown environment", method: "GET", path: "other/loki/api/v1/labels", want: 404},
		{name: "write endpoint", method: "POST", path: "dev/loki/api/v1/push", want: 404},
		{name: "administrative endpoint", method: "GET", path: "dev/loki/api/v1/delete", want: 404},
		{name: "metrics endpoint", method: "GET", path: "dev/metrics", want: 404},
		{name: "wrong method", method: "DELETE", path: "dev/loki/api/v1/query", want: 405},
		{name: "tail requires GET", method: "POST", path: "dev/loki/api/v1/tail", want: 405},
		{name: "invalid query", method: "GET", path: "dev/loki/api/v1/query?query=invalid", want: 400},
		{name: "missing query", method: "GET", path: "dev/loki/api/v1/query_range", want: 400},
		{name: "invalid URL encoding", method: "GET", path: "dev/loki/api/v1/labels?query=%zz", want: 400},
		{name: "JSON body", method: "POST", path: "dev/loki/api/v1/query", body: `{}`, contentType: "application/json", want: 415},
		{name: "oversized form", method: "POST", path: "dev/loki/api/v1/series", body: "match=" + strings.Repeat("a", 1<<20), contentType: "application/x-www-form-urlencoded", want: 400},
		{name: "cross-origin WebSocket", method: "GET", path: "dev/loki/api/v1/tail", upgrade: true, want: 403},
		{name: "WebSocket outside tail", method: "GET", path: "dev/loki/api/v1/labels", upgrade: true, want: 403},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/loki/"+tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", tt.contentType)
			if tt.upgrade {
				req.Header.Set("Connection", "Upgrade")
				req.Header.Set("Upgrade", "websocket")
				req.Header.Set("Origin", "https://other.example.com")
			}
			resp := httptest.NewRecorder()
			proxy.ServeHTTP(resp, req)
			if resp.Code != tt.want {
				t.Errorf("got %d, want %d: %s", resp.Code, tt.want, resp.Body.String())
			}
		})
	}
}

func TestProxyWebSocket(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loki/api/v1/tail" {
			t.Errorf("unexpected upstream path: %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("query"); got != `{app="test", k8s_cluster_name="dev"}` {
			t.Errorf("unexpected query: %q", got)
		}
		if r.Header.Get("X-Scope-OrgID") != "tenant" || r.Header.Get("Authorization") != "" {
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
			target := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/loki/dev/loki/api/v1/tail?query=" + url.QueryEscape(`{app="test"}`)
			headers := http.Header{"Authorization": {"Bearer secret"}}
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
	proxy.ServeHTTP(resp, httptest.NewRequest("GET", "/api/loki/dev/loki/api/v1/labels", nil))
	if resp.Code != http.StatusBadGateway {
		t.Errorf("got %d, want 502", resp.Code)
	}
}

func TestProxyInvalidAddress(t *testing.T) {
	for _, address := range []string{"", "ws://loki:3100", "http://", "http://user:pass@loki:3100", "http://loki:3100?query=override", "http://loki:3100/#fragment"} {
		t.Run(address, func(t *testing.T) {
			if _, err := NewProxy(address, []string{"dev"}, logrus.New()); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func mountedProxy(t *testing.T, address string) http.Handler {
	t.Helper()
	proxy, err := NewProxy(address, []string{"dev"}, logrus.New())
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	router.Mount("/api/loki", proxy)
	return router
}

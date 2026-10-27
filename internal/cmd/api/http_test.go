package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nais/api/internal/auth/authn"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/loki"
	"github.com/nais/api/internal/user"
	"github.com/sirupsen/logrus"
)

func TestLokiProxyAuthentication(t *testing.T) {
	log := logrus.New()
	identity := func(next http.Handler) http.Handler { return next }
	for _, authentication := range []string{"none", "jwt", "kubernetes", "github"} {
		t.Run(authentication, func(t *testing.T) {
			authenticate := func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					next.ServeHTTP(w, r.WithContext(authz.ContextWithActor(r.Context(), &user.User{}, nil)))
				})
			}
			jwt, kubernetes, github := identity, identity, identity
			switch authentication {
			case "jwt":
				jwt = authenticate
			case "kubernetes":
				kubernetes = authenticate
			case "github":
				github = authenticate
			}
			proxy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if authz.ActorFromContext(r.Context()) == nil {
					t.Error("proxy reached without an authenticated actor")
				}
				w.WriteHeader(http.StatusNoContent)
			})
			router := newHTTPRouter(Fakes{}, jwt, kubernetes, github, authn.New(nil, log), nil, proxy, identity)
			for _, websocketRequest := range []bool{false, true} {
				req := httptest.NewRequest("GET", "/api/loki/dev/loki/api/v1/tail", nil)
				if websocketRequest {
					req.Header.Set("Connection", "Upgrade")
					req.Header.Set("Upgrade", "websocket")
				}
				resp := httptest.NewRecorder()
				router.ServeHTTP(resp, req)
				want := http.StatusNoContent
				if authentication == "none" {
					want = http.StatusUnauthorized
				}
				if resp.Code != want {
					t.Errorf("WebSocket=%v: got %d, want %d", websocketRequest, resp.Code, want)
				}
			}
		})
	}
}

func TestLokiProxyWebSocketAuthentication(t *testing.T) {
	log := logrus.New()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/loki/api/v1/tail" {
			conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
			if err != nil {
				t.Error(err)
				return
			}
			defer conn.Close()
			if err := conn.WriteMessage(websocket.TextMessage, []byte("log line")); err != nil {
				t.Error(err)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	proxy, err := loki.NewProxy(upstream.URL, []string{"dev"}, log)
	if err != nil {
		t.Fatal(err)
	}
	authenticate := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Test-Authenticated") == "true" {
				r = r.WithContext(authz.ContextWithActor(r.Context(), &user.User{}, nil))
			}
			next.ServeHTTP(w, r)
		})
	}
	identity := func(next http.Handler) http.Handler { return next }
	router := newHTTPRouter(Fakes{}, authenticate, nil, nil, authn.New(nil, log), nil, proxy, identity)
	server := httptest.NewServer(router)
	defer server.Close()

	for _, authenticated := range []bool{false, true} {
		req := httptest.NewRequest("GET", "/api/loki/dev/loki/api/v1/labels", nil)
		want := http.StatusUnauthorized
		headers := http.Header{}
		if authenticated {
			req.Header.Set("X-Test-Authenticated", "true")
			headers.Set("X-Test-Authenticated", "true")
			want = http.StatusOK
		}
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != want {
			t.Errorf("authenticated=%v: got %d, want %d", authenticated, resp.Code, want)
		}

		target := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/loki/dev/loki/api/v1/tail?query=" + url.QueryEscape(`{app="test"}`)
		conn, respWS, err := websocket.DefaultDialer.DialContext(t.Context(), target, headers)
		if !authenticated {
			if err == nil {
				conn.Close()
				t.Fatal("unauthenticated WebSocket connection succeeded")
			}
			if respWS == nil {
				t.Fatalf("missing HTTP response: %v", err)
			}
			respWS.Body.Close()
			if respWS.StatusCode != http.StatusUnauthorized {
				t.Errorf("unauthenticated WebSocket: got %d, want 401", respWS.StatusCode)
			}
			continue
		}
		if err != nil {
			if respWS != nil {
				respWS.Body.Close()
			}
			t.Fatal(err)
		}
		if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			conn.Close()
			t.Fatal(err)
		}
		_, message, err := conn.ReadMessage()
		conn.Close()
		if err != nil {
			t.Fatal(err)
		}
		if string(message) != "log line" {
			t.Errorf("unexpected message: %q", message)
		}
	}
}

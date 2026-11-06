package loki

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
)

// NewProxy must be mounted behind the API's authentication middleware.
func NewProxy(address string, log logrus.FieldLogger) (http.Handler, error) {
	upstream, err := url.Parse(address)
	if err != nil {
		return nil, fmt.Errorf("parse Loki querier address: %w", err)
	}
	if (upstream.Scheme != "http" && upstream.Scheme != "https") || upstream.Host == "" ||
		upstream.User != nil || upstream.RawQuery != "" || upstream.Fragment != "" {
		return nil, fmt.Errorf("invalid Loki querier address %q", address)
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(upstream)
			r.Out.Header.Del("Authorization")
			r.Out.Header.Del("Cookie")
			r.Out.Header.Del("X-User-Email")
			r.Out.Header.Set("X-Scope-OrgID", "tenant")
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.WithError(err).Error("proxying Loki querier request")
			http.Error(w, "Loki querier unavailable", http.StatusBadGateway)
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") && !sameOrigin(r) {
			http.Error(w, "WebSocket origin not allowed", http.StatusForbidden)
			return
		}
		proxy.ServeHTTP(w, r)
	}), nil
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") &&
		strings.EqualFold(u.Host, r.Host)
}

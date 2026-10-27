package loki

import (
	"fmt"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/grafana/loki/v3/pkg/logql/syntax"
	"github.com/nais/api/internal/environmentmapper"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/sirupsen/logrus"
)

// NewProxy exposes read-only querier endpoints, scoped to configured environments.
// It must be mounted behind the API's authentication middleware.
func NewProxy(address string, clusters []string, log logrus.FieldLogger) (http.Handler, error) {
	upstream, err := url.Parse(address)
	if err != nil {
		return nil, fmt.Errorf("parse Loki querier address: %w", err)
	}
	if (upstream.Scheme != "http" && upstream.Scheme != "https") || upstream.Host == "" ||
		upstream.User != nil || upstream.RawQuery != "" || upstream.Fragment != "" {
		return nil, fmt.Errorf("invalid Loki querier address %q", address)
	}

	environments := make(map[string]string, len(clusters))
	for _, cluster := range clusters {
		environments[environmentmapper.EnvironmentName(cluster)] = cluster
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

	router := chi.NewRouter()
	router.Handle("/{env}/loki/api/v1/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cluster, ok := environments[chi.URLParam(r, "env")]
		if !ok {
			http.Error(w, "Unknown environment", http.StatusNotFound)
			return
		}

		endpoint := chi.URLParam(r, "*")
		if !isQuerierEndpoint(endpoint) {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && (r.Method != http.MethodPost || endpoint == "tail") {
			w.Header().Set("Allow", "GET")
			if endpoint != "tail" {
				w.Header().Set("Allow", "GET, POST")
			}
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			if endpoint != "tail" || !sameOrigin(r) {
				http.Error(w, "WebSocket origin or endpoint not allowed", http.StatusForbidden)
				return
			}
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if r.Method == http.MethodPost {
			contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || contentType != "application/x-www-form-urlencoded" {
				http.Error(w, "Expected a URL-encoded form", http.StatusUnsupportedMediaType)
				return
			}
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid query parameters or form", http.StatusBadRequest)
			return
		}
		if err := scopeProxyParameters(r.Form, endpoint, cluster); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Forward a single canonical parameter set so body/query precedence cannot
		// bypass the environment constraint.
		r.URL.Path = "/loki/api/v1/" + endpoint
		r.URL.RawPath = ""
		r.URL.RawQuery = r.Form.Encode()
		r.Body = http.NoBody
		r.ContentLength = 0
		r.TransferEncoding = nil
		r.Header.Del("Content-Type")
		r.Header.Del("Content-Length")
		proxy.ServeHTTP(w, r)
	}))
	return router, nil
}

func isQuerierEndpoint(endpoint string) bool {
	switch endpoint {
	case "query", "query_range", "tail", "labels", "series",
		"index/stats", "index/volume", "index/volume_range", "index/shards",
		"patterns", "detected_fields", "detected_labels":
		return true
	}
	parts := strings.Split(endpoint, "/")
	return len(parts) == 3 && parts[1] != "" && parts[2] == "values" &&
		(parts[0] == "label" || parts[0] == "detected_field")
}

func scopeProxyParameters(params url.Values, endpoint, cluster string) error {
	if endpoint == "series" {
		selectors := append(params["match[]"], params["match"]...)
		if len(selectors) == 0 {
			selectors = []string{"{}"}
		}
		scoped := make([]string, 0, len(selectors))
		for _, selector := range selectors {
			matchers, err := syntax.ParseMatchers(selector, false)
			if err != nil {
				return fmt.Errorf("invalid stream selector: %w", err)
			}
			expr := &syntax.MatchersExpr{Mts: matchers}
			enforceClusterMatcher(expr, cluster)
			scoped = append(scoped, expr.String())
		}
		params.Del("match")
		params["match[]"] = scoped
		return nil
	}

	query := params.Get("query")
	if query == "" {
		switch endpoint {
		case "labels", "detected_labels", "index/stats", "index/volume", "index/volume_range", "index/shards":
			query = "{}"
		default:
			if strings.HasPrefix(endpoint, "label/") {
				query = "{}"
			} else {
				return fmt.Errorf("query is required")
			}
		}
	}

	// Empty selectors are valid for metadata endpoints but not standalone LogQL.
	// Validate the expression after adding the non-empty environment matcher.
	expr, err := syntax.ParseExprWithoutValidation(query)
	if err != nil {
		return fmt.Errorf("invalid LogQL query: %w", err)
	}
	expr.Walk(func(e syntax.Expr) bool {
		if matchers, ok := e.(*syntax.MatchersExpr); ok {
			enforceClusterMatcher(matchers, cluster)
		}
		return true
	})
	scoped := expr.String()
	if _, err := syntax.ParseExpr(scoped); err != nil {
		return fmt.Errorf("invalid LogQL query: %w", err)
	}
	params.Set("query", scoped)
	return nil
}

func enforceClusterMatcher(expr *syntax.MatchersExpr, cluster string) {
	for _, matcher := range expr.Matchers() {
		if matcher.Name == clusterNameLabel && matcher.Type == labels.MatchEqual && matcher.Value == cluster {
			return
		}
	}
	expr.AppendMatchers([]*labels.Matcher{{
		Type:  labels.MatchEqual,
		Name:  clusterNameLabel,
		Value: cluster,
	}})
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

package postgres

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"

	"github.com/grafana/loki/v3/pkg/logql/syntax"
	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/graph/ident"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

// WithLogScope resolves an optional logical Postgres ID on Loki queries. Physical
// naming and the legacy/direct ownership transition stay inside the API. Requests
// without a Postgres ID retain the existing authenticated Loki proxy contract.
// This handler must be mounted behind the same authentication middleware as that proxy.
func WithLogScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parameters := r.URL.Query()
		if !parameters.Has("postgres") {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet || (r.URL.Path != "/loki/api/v1/query" && r.URL.Path != "/loki/api/v1/query_range") {
			http.Error(w, "Postgres logs require a GET query or query_range request", http.StatusBadRequest)
			return
		}
		var id ident.Ident
		ids, queries := parameters["postgres"], parameters["query"]
		if len(ids) != 1 || ids[0] == "" || len(queries) != 1 || id.UnmarshalGQLContext(r.Context(), ids[0]) != nil || id.Type != "PG" {
			http.Error(w, "A valid Postgres ID and a single log query are required", http.StatusBadRequest)
			return
		}
		if len(id.Parts()) != 3 || slices.Contains(id.Parts(), "") {
			http.Error(w, "Invalid Postgres ID", http.StatusBadRequest)
			return
		}
		pg, err := GetPostgresByIdent(r.Context(), id)
		if err != nil {
			status := http.StatusInternalServerError
			if k8serrors.IsNotFound(err) {
				status = http.StatusNotFound
			}
			http.Error(w, "Unable to resolve Postgres for logs", status)
			return
		}
		cluster, err := physicalClusterName(r.Context(), pg)
		if err != nil {
			http.Error(w, "Unable to resolve Postgres logs", http.StatusInternalServerError)
			return
		}
		if cluster == nil || *cluster == "" {
			http.Error(w, "Postgres logs are not available yet", http.StatusConflict)
			return
		}
		query, err := scopedLogQuery(queries[0], pg, *cluster)
		if err != nil {
			http.Error(w, "Invalid Postgres log query", http.StatusBadRequest)
			return
		}
		parameters.Del("postgres")
		parameters.Set("query", query)
		request := r.Clone(r.Context())
		request.URL.RawQuery = parameters.Encode()
		next.ServeHTTP(w, request)
	})
}

func scopedLogQuery(query string, pg *Postgres, cluster string) (string, error) {
	expr, err := syntax.ParseExpr(query)
	if err != nil {
		return "", err
	}
	// Pod/container names are structured metadata, not necessarily indexed stream
	// labels. Filter them before user pipelines can parse or rewrite those labels.
	scope, err := syntax.ParseExpr(fmt.Sprintf(
		`{service_namespace=%s,service_name=%s,k8s_cluster_name=%s} | k8s_container_name="postgres" | k8s_pod_name=~%s`,
		strconv.Quote(pg.TeamSlug.String()), strconv.Quote(pg.Name),
		strconv.Quote(environmentmapper.ClusterName(pg.EnvironmentName)),
		strconv.Quote("^"+regexp.QuoteMeta(cluster)+"-[0-9]+$"),
	))
	if err != nil {
		return "", err
	}
	pipeline := scope.(*syntax.PipelineExpr)
	if matchers, ok := expr.(*syntax.MatchersExpr); ok {
		expr = &syntax.PipelineExpr{Left: matchers}
	}
	selectors := 0
	expr.Walk(func(e syntax.Expr) bool {
		switch e := e.(type) {
		case *syntax.LogRangeExpr:
			if matchers, ok := e.Left.(*syntax.MatchersExpr); ok {
				e.Left = &syntax.PipelineExpr{Left: matchers}
			}
		case *syntax.PipelineExpr:
			e.MultiStages = append(slices.Clone(pipeline.MultiStages), e.MultiStages...)
		case *syntax.MatchersExpr:
			e.AppendMatchers(pipeline.Matchers())
			selectors++
		}
		return true
	})
	if selectors == 0 {
		return "", fmt.Errorf("Postgres log query must select log streams")
	}
	return expr.String(), nil
}

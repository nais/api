package promclient

import (
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser"
)

func injectEnvToQuery(query, env string) (string, error) {
	expr, err := parser.ParseExpr(query)
	if err != nil {
		return "", err
	}

	addClusterMatcher(expr, env)
	return expr.String(), nil
}

func addClusterMatcher(expr parser.Expr, env string) {
	switch n := expr.(type) {
	case *parser.VectorSelector:
		ensureClusterMatcher(n, env)

	case *parser.AggregateExpr:
		addClusterMatcher(n.Expr, env)

	case *parser.BinaryExpr:
		addClusterMatcher(n.LHS, env)
		addClusterMatcher(n.RHS, env)

	case *parser.Call:
		for _, arg := range n.Args {
			addClusterMatcher(arg, env)
		}

	case *parser.SubqueryExpr:
		addClusterMatcher(n.Expr, env)

	case *parser.ParenExpr:
		addClusterMatcher(n.Expr, env)

	case *parser.UnaryExpr:
		addClusterMatcher(n.Expr, env)

	case *parser.StepInvariantExpr:
		addClusterMatcher(n.Expr, env)
	}
}

func ensureClusterMatcher(vs *parser.VectorSelector, env string) {
	for _, m := range vs.LabelMatchers {
		if m.Name == "k8s_cluster_name" {
			return
		}
	}

	vs.LabelMatchers = append(vs.LabelMatchers, &labels.Matcher{
		Type:  labels.MatchEqual,
		Name:  "k8s_cluster_name",
		Value: env,
	})
}

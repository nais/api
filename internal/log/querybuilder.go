package log

import (
	"fmt"
	"regexp"
	"strings"
)

type QueryBuilder struct {
	selectors []string
	filters   []string
}

func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{
		selectors: make([]string, 0),
		filters:   make([]string, 0),
	}
}

func (qb *QueryBuilder) AddApp(app string) *QueryBuilder {
	qb.selectors = append(qb.selectors, fmt.Sprintf(`service_name="%s"`, app))
	return qb
}

func (qb *QueryBuilder) AddNamespace(namespace string) *QueryBuilder {
	qb.selectors = append(qb.selectors, fmt.Sprintf(`service_namespace="%s"`, namespace))
	return qb
}

func (qb *QueryBuilder) AddCluster(cluster string) *QueryBuilder {
	qb.selectors = append(qb.selectors, fmt.Sprintf(`k8s_cluster_name="%s"`, cluster))
	return qb
}

func (qb *QueryBuilder) AddLevel(level string) *QueryBuilder {
	qb.selectors = append(qb.selectors, fmt.Sprintf(`detected_level="%s"`, level))
	return qb
}

func (qb *QueryBuilder) AddGrep(pattern string) *QueryBuilder {
	escapedPattern := regexp.QuoteMeta(pattern)
	qb.filters = append(qb.filters, fmt.Sprintf(`|~ ".*%s.*"`, escapedPattern))
	return qb
}

func (qb *QueryBuilder) AddRegex(pattern string) *QueryBuilder {
	qb.filters = append(qb.filters, fmt.Sprintf(`|~ "%s"`, pattern))
	return qb
}

func (qb *QueryBuilder) Build() string {
	if len(qb.selectors) == 0 {
		qb.selectors = append(qb.selectors, "service_name!=\"\"") // Fallback
	}

	selectors := fmt.Sprintf("{%s}", strings.Join(qb.selectors, ","))
	filters := strings.Join(qb.filters, " ")

	return strings.Join(append([]string{selectors}, filters), " ")
}

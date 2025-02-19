package logging

import (
	"context"
	"fmt"
	"net/url"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
)

type SupportedLogDestination string

const (
	Loki       SupportedLogDestination = "loki"
	SecureLogs SupportedLogDestination = "secure_logs"
)

func (s SupportedLogDestination) Valid() bool {
	switch s {
	case Loki, SecureLogs:
		return true
	default:
		return false
	}
}

func (s SupportedLogDestination) String() string {
	return string(s)
}

type logDestinationBase struct {
	WorkloadType    workload.Type
	TeamSlug        slug.Slug
	EnvironmentName string
	WorkloadName    string
}

type LogDestination interface {
	IsNode()
	IsLogDestination()
}

type LogDestinationSecureLogs struct {
	logDestinationBase
}

func (LogDestinationSecureLogs) IsLogDestination() {}
func (LogDestinationSecureLogs) IsNode()           {}
func (l LogDestinationSecureLogs) ID() ident.Ident {
	return newIdent(SecureLogs, l.WorkloadType, l.TeamSlug, l.EnvironmentName, l.WorkloadName)
}

type LogDestinationLoki struct {
	logDestinationBase
}

func (LogDestinationLoki) IsLogDestination() {}
func (LogDestinationLoki) IsNode()           {}
func (l LogDestinationLoki) ID() ident.Ident {
	return newIdent(Loki, l.WorkloadType, l.TeamSlug, l.EnvironmentName, l.WorkloadName)
}

func (l LogDestinationLoki) URL(ctx context.Context) string {
	const tpl = `{"datasource":"%s-loki","queries":[{"expr":"{service_name=\"%s\", service_namespace=\"%s\"}"}],"range":true}`

	tenantName := fromContext(ctx).tenantName
	lokiURL := "https://grafana." + tenantName + ".cloud.nais.io/explore?orgId=1&left="

	return lokiURL + url.QueryEscape(fmt.Sprintf(tpl, l.EnvironmentName, l.WorkloadName, l.TeamSlug))
}

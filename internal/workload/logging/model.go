package logging

import (
	"context"
	"fmt"
	"strings"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
)

type SupportedLogDestination string

const (
	Loki       SupportedLogDestination = "loki"
	SecureLogs SupportedLogDestination = "secure_logs"
	Generic    SupportedLogDestination = "generic"
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

type LogDestinationGeneric struct {
	logDestinationBase

	Name string
}

func (LogDestinationGeneric) IsLogDestination() {}
func (LogDestinationGeneric) IsNode()           {}
func (l LogDestinationGeneric) ID() ident.Ident {
	return newGenericIdent(Generic, l.WorkloadType, l.TeamSlug, l.EnvironmentName, l.WorkloadName, l.Name)
}

type LogDestinationLoki struct {
	logDestinationBase
}

func (LogDestinationLoki) IsLogDestination() {}
func (LogDestinationLoki) IsNode()           {}
func (l LogDestinationLoki) ID() ident.Ident {
	return newIdent(Loki, l.WorkloadType, l.TeamSlug, l.EnvironmentName, l.WorkloadName)
}

func (l LogDestinationLoki) GrafanaURL(ctx context.Context) string {
	const tpl = `var-ds=%s-loki&var-filters=service_name|%%3D|%s&var-filters=service_namespace|%%3D|%s`

	tenantName := fromContext(ctx).tenantName
	envName := l.EnvironmentName
	// All loki logs are stored in gcp, update the envName to match the loki datasource
	if before, ok := strings.CutSuffix(envName, "-fss"); ok {
		envName = before + "-gcp"
	}
	lokiURL := "https://grafana." + tenantName + ".cloud.nais.io/a/grafana-lokiexplore-app/explore/service/" + l.WorkloadName + "/logs?"

	return lokiURL + fmt.Sprintf(tpl, envName, l.WorkloadName, l.TeamSlug)
}

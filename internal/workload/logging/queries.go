package logging

import (
	"context"

	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/workload"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
)

func FromWorkload(ctx context.Context, wl workload.Workload) []LogDestination {
	logging := wl.GetLogging()
	var logDestinations []nais_io_v1.LogDestination
	if logging == nil || logging.Enabled {
		if logging != nil {
			logDestinations = logging.Destinations
		}
		if len(logDestinations) == 0 {
			destination := fromContext(ctx).environmentDestinations[environmentmapper.ClusterName(wl.GetEnvironmentName())]
			if destination == "" {
				destination = Loki
			}
			logDestinations = []nais_io_v1.LogDestination{{ID: string(destination)}}
		}
	}

	base := logDestinationBase{
		WorkloadType:    wl.GetType(),
		TeamSlug:        wl.GetTeamSlug(),
		EnvironmentName: wl.GetEnvironmentName(),
		WorkloadName:    wl.GetName(),
	}

	destinations := make([]LogDestination, 0, len(logDestinations)+1)
	hasSecureLogs := false
	for _, logDestination := range logDestinations {
		switch SupportedLogDestination(logDestination.ID) {
		case SecureLogs:
			destinations = append(destinations, LogDestinationSecureLogs{base})
			hasSecureLogs = true
		case Loki:
			destinations = append(destinations, LogDestinationLoki{base})
		default:
			// Unknown log destination
			destinations = append(destinations, LogDestinationGeneric{
				logDestinationBase: base,
				Name:               logDestination.ID,
			})
		}
	}
	if !hasSecureLogs && allowsTeamLogs(wl) {
		destinations = append(destinations, LogDestinationSecureLogs{base})
	}

	return destinations
}

func allowsTeamLogs(wl workload.Workload) bool {
	policy := wl.GetAccessPolicy()
	if policy == nil || policy.Outbound == nil {
		return false
	}

	clusterName := environmentmapper.ClusterName(wl.GetEnvironmentName())
	for _, rule := range policy.Outbound.Rules {
		namespace := rule.Namespace
		if namespace == "" {
			namespace = wl.GetTeamSlug().String()
		}
		if rule.Application == "logging" && namespace == "nais-system" && rule.MatchesCluster(clusterName) {
			return true
		}
	}
	return false
}

func getByIdent(ctx context.Context, id ident.Ident) (LogDestination, error) {
	kind, workloadType, teamSlug, environment, workloadName, logName, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	base := logDestinationBase{
		WorkloadType:    workload.Type(workloadType),
		TeamSlug:        teamSlug,
		EnvironmentName: environment,
		WorkloadName:    workloadName,
	}

	switch kind {
	case Loki:
		return LogDestinationLoki{base}, nil
	case SecureLogs:
		return LogDestinationSecureLogs{base}, nil
	case Generic:
		return LogDestinationGeneric{
			logDestinationBase: base,
			Name:               logName,
		}, nil
	default:
		return nil, apierror.Errorf("Unknown log destination: %q.", kind)
	}
}

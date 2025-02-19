package logging

import (
	"context"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/workload"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	"github.com/sirupsen/logrus"
)

func FromWorkload(ctx context.Context, wl workload.Workload) []LogDestination {
	logging := wl.GetLogging()
	if logging == nil {
		logging = &nais_io_v1.Logging{
			Enabled: true,
		}

		for _, dl := range fromContext(ctx).defaultLogDestinations {
			logging.Destinations = append(logging.Destinations, nais_io_v1.LogDestination{
				ID: string(dl),
			})
		}
	}

	if !logging.Enabled {
		// Log destinations are disabled
		return []LogDestination{}
	}

	base := logDestinationBase{
		WorkloadType:    wl.GetType(),
		TeamSlug:        wl.GetTeamSlug(),
		EnvironmentName: wl.GetEnvironmentName(),
		WorkloadName:    wl.GetName(),
	}

	var destinations []LogDestination
	for _, logDestination := range logging.Destinations {
		switch SupportedLogDestination(logDestination.ID) {
		case Loki:
			destinations = append(destinations, LogDestinationLoki{base})
		case SecureLogs:
			destinations = append(destinations, LogDestinationSecureLogs{base})
		default:
			// Unknown log destination
			logrus.WithField("destination", logDestination.ID).Error("Unknown log destination")
		}
	}

	return destinations
}

func getByIdent(ctx context.Context, id ident.Ident) (LogDestination, error) {
	kind, workloadType, teamSlug, environment, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	base := logDestinationBase{
		WorkloadType:    workload.Type(workloadType),
		TeamSlug:        teamSlug,
		EnvironmentName: environment,
		WorkloadName:    name,
	}

	switch kind {
	case Loki:
		return LogDestinationLoki{base}, nil
	case SecureLogs:
		return LogDestinationSecureLogs{base}, nil
	default:
		return nil, apierror.Errorf("Unknown log destination: %q.", kind)
	}
}

package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/persistence/aivencredentials"
)

func (r *credentialsActivityLogEntryDataResolver) ServiceType(ctx context.Context, obj *aivencredentials.CredentialsActivityLogEntryData) (string, error) {
	panic(fmt.Errorf("not implemented: ServiceType - serviceType"))
}

func (r *credentialsActivityLogEntryDataResolver) InstanceName(ctx context.Context, obj *aivencredentials.CredentialsActivityLogEntryData) (*string, error) {
	panic(fmt.Errorf("not implemented: InstanceName - instanceName"))
}

func (r *Resolver) CredentialsActivityLogEntryData() gengql.CredentialsActivityLogEntryDataResolver {
	return &credentialsActivityLogEntryDataResolver{r}
}

type credentialsActivityLogEntryDataResolver struct{ *Resolver }

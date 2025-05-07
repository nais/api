package graph

import (
	"context"
	"fmt"
	"time"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/maintenance"
	"github.com/nais/api/internal/persistence/valkey"
	"github.com/nais/api/internal/serviceaccount"
)

func (r *maintenanceResolver) Updates(ctx context.Context, obj *maintenance.Maintenance) ([]context.Context, error) {
	panic(fmt.Errorf("not implemented: Updates - updates"))
}

func (r *updateResolver) Deadline(ctx context.Context, obj *func(ctx context.Context, input serviceaccount.UpdateServiceAccountInput) (*serviceaccount.ServiceAccount, error)) (*time.Time, error) {
	panic(fmt.Errorf("not implemented: Deadline - deadline"))
}

func (r *updateResolver) Title(ctx context.Context, obj *func(ctx context.Context, input serviceaccount.UpdateServiceAccountInput) (*serviceaccount.ServiceAccount, error)) (string, error) {
	panic(fmt.Errorf("not implemented: Title - title"))
}

func (r *updateResolver) Description(ctx context.Context, obj *func(ctx context.Context, input serviceaccount.UpdateServiceAccountInput) (*serviceaccount.ServiceAccount, error)) (string, error) {
	panic(fmt.Errorf("not implemented: Description - description"))
}

func (r *updateResolver) DocumentationLink(ctx context.Context, obj *func(ctx context.Context, input serviceaccount.UpdateServiceAccountInput) (*serviceaccount.ServiceAccount, error)) (*string, error) {
	panic(fmt.Errorf("not implemented: DocumentationLink - documentation_link"))
}

func (r *updateResolver) StartAfter(ctx context.Context, obj *func(ctx context.Context, input serviceaccount.UpdateServiceAccountInput) (*serviceaccount.ServiceAccount, error)) (*time.Time, error) {
	panic(fmt.Errorf("not implemented: StartAfter - start_after"))
}

func (r *updateResolver) StartAt(ctx context.Context, obj *func(ctx context.Context, input serviceaccount.UpdateServiceAccountInput) (*serviceaccount.ServiceAccount, error)) (*time.Time, error) {
	panic(fmt.Errorf("not implemented: StartAt - start_at"))
}

func (r *valkeyInstanceResolver) Maintenance(ctx context.Context, obj *valkey.ValkeyInstance) (*maintenance.Maintenance, error) {
	panic(fmt.Errorf("not implemented: Maintenance - maintenance"))
}

func (r *Resolver) Maintenance() gengql.MaintenanceResolver { return &maintenanceResolver{r} }

func (r *Resolver) Update() gengql.UpdateResolver { return &updateResolver{r} }

type (
	maintenanceResolver struct{ *Resolver }
	updateResolver      struct{ *Resolver }
)

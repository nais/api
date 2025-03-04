// Code generated by sqlc. DO NOT EDIT.

package reconcilersql

import (
	"context"

	"github.com/google/uuid"
)

type Querier interface {
	Configure(ctx context.Context, arg ConfigureParams) error
	Count(ctx context.Context) (int64, error)
	Disable(ctx context.Context, name string) (*Reconciler, error)
	Enable(ctx context.Context, name string) (*Reconciler, error)
	Get(ctx context.Context, name string) (*Reconciler, error)
	GetConfig(ctx context.Context, arg GetConfigParams) ([]*GetConfigRow, error)
	GetReconcilerErrorByID(ctx context.Context, id uuid.UUID) (*ReconcilerError, error)
	List(ctx context.Context, arg ListParams) ([]*Reconciler, error)
	ListByNames(ctx context.Context, names []string) ([]*Reconciler, error)
	ListEnabledReconcilers(ctx context.Context) ([]*Reconciler, error)
	ListReconcilerErrors(ctx context.Context, arg ListReconcilerErrorsParams) ([]*ReconcilerError, error)
	ListReconcilerErrorsCount(ctx context.Context, reconciler string) (int64, error)
}

var _ Querier = (*Queries)(nil)

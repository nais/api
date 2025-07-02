package reconciler

import (
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/reconciler/reconcilersql"
	"github.com/nais/api/internal/slug"
)

type (
	ReconcilerConnection      = pagination.Connection[*Reconciler]
	ReconcilerEdge            = pagination.Edge[*Reconciler]
	ReconcilerErrorConnection = pagination.Connection[*ReconcilerError]
	ReconcilerErrorEdge       = pagination.Edge[*ReconcilerError]
)

type Reconciler struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

func (Reconciler) IsNode()           {}
func (Reconciler) IsActivityLogger() {}

func (r Reconciler) ID() ident.Ident {
	return newReconcilerIdent(r.Name)
}

func toGraphReconciler(r *reconcilersql.Reconciler) *Reconciler {
	return &Reconciler{
		Name:        r.Name,
		DisplayName: r.DisplayName,
		Description: r.Description,
		Enabled:     r.Enabled,
	}
}

type ReconcilerConfig struct {
	Key            string  `json:"key"`
	DisplayName    string  `json:"displayName"`
	Description    string  `json:"description"`
	Configured     bool    `json:"configured"`
	Secret         bool    `json:"secret"`
	Value          *string `json:"value,omitempty"`
	ReconcilerName string  `json:"-"`
}

func toGraphReconcilerConfig(reconcilerName string, u *reconcilersql.GetConfigRow) *ReconcilerConfig {
	return &ReconcilerConfig{
		ReconcilerName: reconcilerName,
		Key:            u.Key,
		DisplayName:    u.DisplayName,
		Description:    u.Description,
		Configured:     u.Configured,
		Secret:         u.Secret,
		Value:          u.Value,
	}
}

type ReconcilerConfigInput struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ReconcilerError struct {
	CorrelationID string    `json:"correlationID"`
	CreatedAt     time.Time `json:"createdAt"`
	Message       string    `json:"message"`
	TeamSlug      slug.Slug `json:"-"`
	UUID          uuid.UUID `json:"-"`
}

func (ReconcilerError) IsNode() {}
func (e *ReconcilerError) ID() ident.Ident {
	return newReconcilerErrorIdent(e.UUID)
}

func toGraphReconcilerError(row *reconcilersql.ReconcilerError) *ReconcilerError {
	return &ReconcilerError{
		CorrelationID: row.CorrelationID.String(),
		CreatedAt:     row.CreatedAt.Time,
		Message:       row.ErrorMessage,
		TeamSlug:      row.TeamSlug,
		UUID:          row.ID,
	}
}

type ConfigureReconcilerInput struct {
	Name   string                   `json:"name"`
	Config []*ReconcilerConfigInput `json:"config"`
}

type DisableReconcilerInput struct {
	Name string `json:"name"`
}

type EnableReconcilerInput struct {
	Name string `json:"name"`
}

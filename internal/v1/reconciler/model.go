package reconciler

import (
	"time"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/reconciler/reconcilersql"
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

func (Reconciler) IsNode() {}

func (r Reconciler) ID() ident.Ident {
	return newIdent(r.Name)
}

func toGraphReconciler(r *reconcilersql.Reconciler) *Reconciler {
	return &Reconciler{
		Name:        r.Name,
		DisplayName: r.DisplayName,
		Description: r.Description,
		Enabled:     r.Enabled,
	}
}

// Reconciler configuration type.
type ReconcilerConfig struct {
	// Configuration key.
	Key string `json:"key"`
	// The human-friendly name of the configuration key.
	DisplayName string `json:"displayName"`
	// Configuration description.
	Description string `json:"description"`
	// Whether or not the configuration key has a value.
	Configured bool `json:"configured"`
	// Whether or not the configuration value is considered a secret. Secret values will not be exposed through the API.
	Secret bool `json:"secret"`
	// Configuration value. This will be set to null if the value is considered a secret.
	Value *string `json:"value,omitempty"`

	// Reconciler name.
	ReconcilerName string `json:"-"`
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

// Reconciler configuration input.
type ReconcilerConfigInput struct {
	// Configuration key.
	Key string `json:"key"`
	// Configuration value.
	Value string `json:"value"`
}

type ReconcilerError struct {
	CorrelationID string    `json:"correlationID"`
	CreatedAt     time.Time `json:"createdAt"`
	Message       string    `json:"message"`
	TeamSlug      slug.Slug `json:"-"`
}

func toGraphReconcilerError(row *reconcilersql.ReconcilerError) *ReconcilerError {
	return &ReconcilerError{
		CorrelationID: row.CorrelationID.String(),
		CreatedAt:     row.CreatedAt.Time,
		Message:       row.ErrorMessage,
		TeamSlug:      row.TeamSlug,
	}
}

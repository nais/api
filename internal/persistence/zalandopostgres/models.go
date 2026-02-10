package zalandopostgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/validate"
	"github.com/nais/api/internal/workload"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ZalandoPostgres struct {
	Name              string              `json:"name"`
	EnvironmentName   string              `json:"-"`
	WorkloadReference *workload.Reference `json:"-"`
	TeamSlug          slug.Slug           `json:"-"`
}

type GrantZalandoPostgresAccessInput struct {
	ClusterName     string    `json:"clusterName"`
	TeamSlug        slug.Slug `json:"teamSlug"`
	EnvironmentName string    `json:"environmentName"`
	Grantee         string    `json:"grantee"`
	Duration        string    `json:"duration"`
}

func (i *GrantZalandoPostgresAccessInput) Validate(ctx context.Context) error {
	return i.ValidationErrors(ctx).NilIfEmpty()
}

func (i *GrantZalandoPostgresAccessInput) ValidationErrors(ctx context.Context) *validate.ValidationErrors {
	verr := validate.New()
	i.ClusterName = strings.TrimSpace(i.ClusterName)
	i.EnvironmentName = strings.TrimSpace(i.EnvironmentName)

	if i.ClusterName == "" {
		verr.Add("clusterName", "ClusterName must not be empty.")
	}
	if i.EnvironmentName == "" {
		verr.Add("environmentName", "Environment name must not be empty.")
	}
	if i.TeamSlug == "" {
		verr.Add("teamSlug", "Team slug must not be empty.")
	}
	if i.Grantee == "" {
		verr.Add("grantee", "Grantee must not be empty.")
	}

	duration, err := time.ParseDuration(i.Duration)
	if err != nil {
		verr.Add("duration", "%s", err)
	} else if duration > 4*time.Hour {
		verr.Add("duration", "Duration \"%s\" is out-of-bounds. Must be less than 4 hours.", i.Duration)
	}

	_, err = GetZalandoPostgres(ctx, i.TeamSlug, i.EnvironmentName, i.ClusterName)
	if err != nil {
		if errors.Is(err, &watcher.ErrorNotFound{}) {
			verr.Add("clusterName", "Could not find zalando postgres cluster named \"%s\"", i.ClusterName)
		} else {
			verr.Add("clusterName", "%s", err)
		}
	}

	return verr
}

type GrantZalandoPostgresAccessPayload struct {
	Error *string `json:"error,omitempty"`
}

func (ZalandoPostgres) IsPersistence() {}
func (ZalandoPostgres) IsSearchNode()  {}
func (ZalandoPostgres) IsNode()        {}

func (p *ZalandoPostgres) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (p *ZalandoPostgres) DeepCopyObject() runtime.Object {
	return p
}

func (p *ZalandoPostgres) GetName() string {
	return p.Name
}

func (p *ZalandoPostgres) GetNamespace() string {
	return p.TeamSlug.String()
}

func (p *ZalandoPostgres) GetLabels() map[string]string {
	return nil
}

func (p *ZalandoPostgres) ID() ident.Ident {
	return newIdent(p.TeamSlug, p.EnvironmentName, p.Name)
}

func toZalandoPostgres(u *unstructured.Unstructured, environmentName string) (*ZalandoPostgres, error) {
	return &ZalandoPostgres{
		Name:            u.GetName(),
		EnvironmentName: environmentName,
		TeamSlug:        slug.Slug(u.GetNamespace()),
	}, nil
}

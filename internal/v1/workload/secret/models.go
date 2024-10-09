package secret

import (
	"time"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type (
	SecretConnection = pagination.Connection[*Secret]
	SecretEdge       = pagination.Edge[*Secret]
)

type Secret struct {
	Name                string     `json:"name"`
	LastModifiedAt      *time.Time `json:"lastModifiedAt,omitempty"`
	ModifiedByUserEmail *string    `json:"lastModifiedBy,omitempty"`

	TeamSlug        slug.Slug `json:"-"`
	EnvironmentName string    `json:"-"`
}

func (s *Secret) ID() ident.Ident {
	return newIdent(s.TeamSlug, s.EnvironmentName, s.Name)
}

func (Secret) IsNode() {}

func (s *Secret) GetName() string {
	return s.Name
}

func (s *Secret) GetNamespace() string {
	return s.TeamSlug.String()
}

func (s *Secret) GetLabels() map[string]string {
	return nil
}

func (s *Secret) DeepCopyObject() runtime.Object {
	return s
}

func (s *Secret) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func toGraphSecret(o *unstructured.Unstructured, environmentName string) *Secret {
	return &Secret{
		Name:            o.GetName(),
		TeamSlug:        slug.Slug(o.GetNamespace()),
		EnvironmentName: environmentName,
	}
}

type SecretVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

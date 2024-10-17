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

const (
	secretLabelManagedByKey        = "nais.io/managed-by"
	secretLabelManagedByVal        = "console"
	secretAnnotationLastModifiedAt = "console.nais.io/last-modified-at"
	secretAnnotationLastModifiedBy = "console.nais.io/last-modified-by"
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

type CreateSecretInput struct {
	Name        string    `json:"name"`
	Environment string    `json:"environment"`
	Team        slug.Slug `json:"team"`
}

type CreateSecretPayload struct {
	Secret *Secret `json:"secret"`
}

type UpdateSecretInput struct {
	Name        string                 `json:"name"`
	Environment string                 `json:"environment"`
	Team        slug.Slug              `json:"team"`
	Data        []*SecretVariableInput `json:"data"`
}

type UpdateSecretPayload struct {
	Secret *Secret `json:"secret"`
}

type SecretVariableInput struct {
	Name  string `json:"name"`
	Value string `json:"value"`
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

func toGraphSecret(o *unstructured.Unstructured, environmentName string) (*Secret, bool) {
	if !secretIsManagedByConsole(o) {
		return nil, false
	}

	var lastModifiedAt *time.Time
	if t, ok := o.GetAnnotations()[secretAnnotationLastModifiedAt]; ok {
		tm, err := time.Parse(time.RFC3339, t)
		if err == nil {
			lastModifiedAt = &tm
		}
	}

	var lastModifiedBy *string
	if email, ok := o.GetAnnotations()[secretAnnotationLastModifiedBy]; ok {
		lastModifiedBy = &email
	}

	return &Secret{
		Name:                o.GetName(),
		TeamSlug:            slug.Slug(o.GetNamespace()),
		EnvironmentName:     environmentName,
		LastModifiedAt:      lastModifiedAt,
		ModifiedByUserEmail: lastModifiedBy,
	}, true
}

type SecretValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type DeleteSecretInput struct {
	Name        string    `json:"name"`
	Environment string    `json:"environment"`
	Team        slug.Slug `json:"team"`
}

type DeleteSecretPayload struct {
	SecretDeleted bool `json:"secretDeleted"`
}

type RemoveSecretValueInput struct {
	SecretName  string    `json:"secretName"`
	Environment string    `json:"environment"`
	Team        slug.Slug `json:"team"`
	ValueName   string    `json:"valueName"`
}

type RemoveSecretValuePayload struct {
	Secret *Secret `json:"secret,omitempty"`
}

type SecretValueInput struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type SetSecretValueInput struct {
	Name        string            `json:"name"`
	Environment string            `json:"environment"`
	Team        slug.Slug         `json:"team"`
	Value       *SecretValueInput `json:"value"`
}

type SetSecretValuePayload struct {
	Secret *Secret `json:"secret,omitempty"`
}

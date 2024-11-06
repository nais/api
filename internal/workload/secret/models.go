package secret

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
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
	LastModifiedAt      *time.Time `json:"lastModifiedAt"`
	ModifiedByUserEmail *string    `json:"lastModifiedBy"`

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

type SecretValueInput struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type AddSecretValueInput struct {
	Name        string            `json:"name"`
	Environment string            `json:"environment"`
	Team        slug.Slug         `json:"team"`
	Value       *SecretValueInput `json:"value"`
}

type UpdateSecretValueInput struct {
	Name        string            `json:"name"`
	Environment string            `json:"environment"`
	Team        slug.Slug         `json:"team"`
	Value       *SecretValueInput `json:"value"`
}

type RemoveSecretValueInput struct {
	SecretName  string    `json:"secretName"`
	Environment string    `json:"environment"`
	Team        slug.Slug `json:"team"`
	ValueName   string    `json:"valueName"`
}

type AddSecretValuePayload struct {
	Secret *Secret `json:"secret"`
}

type UpdateSecretValuePayload struct {
	Secret *Secret `json:"secret"`
}

type RemoveSecretValuePayload struct {
	Secret *Secret `json:"secret"`
}

type SecretOrder struct {
	// The field to order items by.
	Field SecretOrderField `json:"field"`
	// The direction to order items by.
	Direction model.OrderDirection `json:"direction"`
}

type SecretOrderField string

const (
	// Order secrets by name.
	SecretOrderFieldName SecretOrderField = "NAME"
	// Order secrets by the name of the environment.
	SecretOrderFieldEnvironment SecretOrderField = "ENVIRONMENT"
	// Order secrets by the last time it was modified.
	SecretOrderFieldLastModifiedAt SecretOrderField = "LAST_MODIFIED_AT"
)

var AllSecretOrderField = []SecretOrderField{
	SecretOrderFieldName,
	SecretOrderFieldEnvironment,
	SecretOrderFieldLastModifiedAt,
}

func (e SecretOrderField) IsValid() bool {
	switch e {
	case SecretOrderFieldName, SecretOrderFieldEnvironment, SecretOrderFieldLastModifiedAt:
		return true
	}
	return false
}

func (e SecretOrderField) String() string {
	return string(e)
}

func (e *SecretOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = SecretOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid SecretOrderField", str)
	}
	return nil
}

func (e SecretOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

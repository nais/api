package config

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload/secret"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type (
	ConfigConnection = pagination.Connection[*Config]
	ConfigEdge       = pagination.Edge[*Config]
)

type Config struct {
	Name                string            `json:"name"`
	LastModifiedAt      *time.Time        `json:"lastModifiedAt"`
	ModifiedByUserEmail *string           `json:"lastModifiedBy"`
	Data                map[string]string `json:"-"` // ConfigMap data cached as-is (not sensitive)
	BinaryData          map[string]string `json:"-"` // ConfigMap binaryData cached as base64 strings

	TeamSlug        slug.Slug `json:"-"`
	EnvironmentName string    `json:"-"`
}

type CreateConfigInput struct {
	Name            string    `json:"name"`
	EnvironmentName string    `json:"environmentName"`
	TeamSlug        slug.Slug `json:"teamSlug"`
}

type CreateConfigPayload struct {
	Config *Config `json:"config"`
}

type UpdateConfigInput struct {
	Name            string              `json:"name"`
	EnvironmentName string              `json:"environmentName"`
	TeamSlug        slug.Slug           `json:"teamSlug"`
	Data            []*ConfigValueInput `json:"data"`
}

type UpdateConfigPayload struct {
	Config *Config `json:"config"`
}

type ConfigValueInput struct {
	Name     string                `json:"name"`
	Value    string                `json:"value"`
	Encoding *secret.ValueEncoding `json:"encoding"`
}

func (c *Config) ID() ident.Ident {
	return newIdent(c.TeamSlug, c.EnvironmentName, c.Name)
}

func (Config) IsNode() {}

func (c *Config) GetName() string {
	return c.Name
}

func (c *Config) GetNamespace() string {
	return c.TeamSlug.String()
}

func (c *Config) GetLabels() map[string]string {
	return nil
}

func (c *Config) DeepCopyObject() runtime.Object {
	return c
}

func (c *Config) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func toGraphConfig(o *unstructured.Unstructured, environmentName string) (*Config, bool) {
	if !configIsManagedByConsole(o) {
		return nil, false
	}

	var lastModifiedAt *time.Time
	if t, ok := o.GetAnnotations()[kubernetes.AnnotationLastModifiedAt]; ok {
		tm, err := time.Parse(time.RFC3339, t)
		if err == nil {
			lastModifiedAt = &tm
		}
	}

	var lastModifiedBy *string
	if email, ok := o.GetAnnotations()[kubernetes.AnnotationLastModifiedBy]; ok {
		lastModifiedBy = &email
	}

	// ConfigMap data is not sensitive, so we keep it as-is
	data, _, _ := unstructured.NestedStringMap(o.Object, "data")
	binaryData, _, _ := unstructured.NestedStringMap(o.Object, "binaryData")

	return &Config{
		Name:                o.GetName(),
		TeamSlug:            slug.Slug(o.GetNamespace()),
		EnvironmentName:     environmentName,
		LastModifiedAt:      lastModifiedAt,
		ModifiedByUserEmail: lastModifiedBy,
		Data:                data,
		BinaryData:          binaryData,
	}, true
}

type ConfigValue struct {
	Name     string               `json:"name"`
	Value    string               `json:"value"`
	Encoding secret.ValueEncoding `json:"encoding"`
}

type DeleteConfigInput struct {
	Name            string    `json:"name"`
	EnvironmentName string    `json:"environmentName"`
	TeamSlug        slug.Slug `json:"teamSlug"`
}

type DeleteConfigPayload struct {
	ConfigDeleted bool `json:"configDeleted"`
}

type AddConfigValueInput struct {
	Name            string            `json:"name"`
	EnvironmentName string            `json:"environmentName"`
	TeamSlug        slug.Slug         `json:"teamSlug"`
	Value           *ConfigValueInput `json:"value"`
}

type UpdateConfigValueInput struct {
	Name            string            `json:"name"`
	EnvironmentName string            `json:"environmentName"`
	TeamSlug        slug.Slug         `json:"teamSlug"`
	Value           *ConfigValueInput `json:"value"`
}

type RemoveConfigValueInput struct {
	ConfigName      string    `json:"configName"`
	EnvironmentName string    `json:"environmentName"`
	TeamSlug        slug.Slug `json:"teamSlug"`
	ValueName       string    `json:"valueName"`
}

type AddConfigValuePayload struct {
	Config *Config `json:"config"`
}

type UpdateConfigValuePayload struct {
	Config *Config `json:"config"`
}

type RemoveConfigValuePayload struct {
	Config *Config `json:"config"`
}

type ConfigOrder struct {
	Field     ConfigOrderField     `json:"field"`
	Direction model.OrderDirection `json:"direction"`
}

type ConfigOrderField string

func (e ConfigOrderField) IsValid() bool {
	return SortFilter.SupportsSort(e)
}

func (e ConfigOrderField) String() string {
	return string(e)
}

func (e *ConfigOrderField) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ConfigOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ConfigOrderField", str)
	}
	return nil
}

func (e ConfigOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type ConfigFilter struct {
	Name  *string `json:"name"`
	InUse *bool   `json:"inUse"`
}

// IsActivityLogger implements the ActivityLogger interface.
func (Config) IsActivityLogger() {}

type TeamInventoryCountConfigs struct {
	Total int `json:"total"`
}

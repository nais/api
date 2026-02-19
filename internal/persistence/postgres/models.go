package postgres

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/validate"
	"github.com/nais/api/internal/workload"
	data_nais_io_v1 "github.com/nais/liberator/pkg/apis/data.nais.io/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type (
	PostgresInstanceConnection = pagination.Connection[*PostgresInstance]
	PostgresInstanceEdge       = pagination.Edge[*PostgresInstance]
)

type PostgresInstance struct {
	Name              string                     `json:"name"`
	EnvironmentName   string                     `json:"-"`
	WorkloadReference *workload.Reference        `json:"-"`
	TeamSlug          slug.Slug                  `json:"-"`
	Resources         *PostgresInstanceResources `json:"resources"`
	MajorVersion      string                     `json:"majorVersion"`
	Audit             PostgresInstanceAudit      `json:"audit"`
}

type PostgresInstanceAudit struct {
	Enabled         bool      `json:"enabled"`
	TeamSlug        slug.Slug `json:"-"`
	EnvironmentName string    `json:"-"`
	InstanceName    string    `json:"-"`
}

func (PostgresInstance) IsPersistence() {}

func (PostgresInstance) IsNode() {}

func (PostgresInstance) IsSearchNode() {}

type PostgresInstanceResources struct {
	CPU      string `json:"cpu"`
	Memory   string `json:"memory"`
	DiskSize string `json:"diskSize"`
}

type GrantPostgresAccessInput struct {
	ClusterName     string    `json:"clusterName"`
	TeamSlug        slug.Slug `json:"teamSlug"`
	EnvironmentName string    `json:"environmentName"`
	Grantee         string    `json:"grantee"`
	Duration        string    `json:"duration"`
}

func (i *GrantPostgresAccessInput) Validate(ctx context.Context) error {
	return i.ValidationErrors(ctx).NilIfEmpty()
}

func (i *GrantPostgresAccessInput) ValidationErrors(ctx context.Context) *validate.ValidationErrors {
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
			verr.Add("clusterName", "Could not find postgres cluster named \"%s\"", i.ClusterName)
		} else {
			verr.Add("clusterName", "%s", err)
		}
	}

	return verr
}

type GrantPostgresAccessPayload struct {
	Error *string `json:"error,omitempty"`
}

func (p *PostgresInstance) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (p *PostgresInstance) DeepCopyObject() runtime.Object {
	return p
}

func (p *PostgresInstance) GetName() string {
	return p.Name
}

func (p *PostgresInstance) GetNamespace() string {
	return p.TeamSlug.String()
}

func (p *PostgresInstance) GetLabels() map[string]string {
	return nil
}

func (p *PostgresInstance) ID() ident.Ident {
	return newIdent(p.TeamSlug, p.EnvironmentName, p.Name)
}

func toPostgres(u *unstructured.Unstructured, environmentName string) (*PostgresInstance, error) {
	obj := &data_nais_io_v1.Postgres{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("converting to Postgres: %w", err)
	}

	audit := false
	if obj.Spec.Cluster.Audit != nil {
		audit = obj.Spec.Cluster.Audit.Enabled
	}

	return &PostgresInstance{
		Name:              obj.GetName(),
		EnvironmentName:   environmentName,
		TeamSlug:          slug.Slug(obj.GetNamespace()),
		WorkloadReference: workload.ReferenceFromOwnerReferences(obj.GetOwnerReferences()),
		Resources: &PostgresInstanceResources{
			CPU:      obj.Spec.Cluster.Resources.Cpu.String(),
			Memory:   obj.Spec.Cluster.Resources.Memory.String(),
			DiskSize: obj.Spec.Cluster.Resources.DiskSize.String(),
		},
		MajorVersion: obj.Spec.Cluster.MajorVersion,
		Audit: PostgresInstanceAudit{
			Enabled:         audit,
			TeamSlug:        slug.Slug(obj.GetNamespace()),
			EnvironmentName: environmentName,
			InstanceName:    obj.GetName(),
		},
	}, nil
}

type PostgresInstanceOrder struct {
	Field     PostgresInstanceOrderField `json:"field"`
	Direction model.OrderDirection       `json:"direction"`
}

type PostgresInstanceOrderField string

const (
	PostgresInstanceOrderFieldName        PostgresInstanceOrderField = "NAME"
	PostgresInstanceOrderFieldEnvironment PostgresInstanceOrderField = "ENVIRONMENT"
)

var AllPostgresInstanceOrderField = []PostgresInstanceOrderField{
	PostgresInstanceOrderFieldName,
	PostgresInstanceOrderFieldEnvironment,
}

func (e PostgresInstanceOrderField) IsValid() bool {
	switch e {
	case PostgresInstanceOrderFieldName, PostgresInstanceOrderFieldEnvironment:
		return true
	}
	return false
}

func (e PostgresInstanceOrderField) String() string {
	return string(e)
}

func (e *PostgresInstanceOrderField) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = PostgresInstanceOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid PostgresInstanceOrderField", str)
	}
	return nil
}

func (e PostgresInstanceOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

func (e *PostgresInstanceOrderField) UnmarshalJSON(b []byte) error {
	s, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}
	return e.UnmarshalGQL(s)
}

func (e PostgresInstanceOrderField) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	e.MarshalGQL(&buf)
	return buf.Bytes(), nil
}

type TeamInventoryCountPostgresInstances struct {
	Total int `json:"total"`
}

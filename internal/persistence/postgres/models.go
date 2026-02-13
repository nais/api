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
	// Indicates whether audit logging is enabled for the Postgres cluster.
	Enabled bool `json:"enabled"`
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
	cpu, _, _ := unstructured.NestedString(u.Object, "spec", "cluster", "resources", "cpu")
	memory, _, _ := unstructured.NestedString(u.Object, "spec", "cluster", "resources", "memory")
	diskSize, _, _ := unstructured.NestedString(u.Object, "spec", "cluster", "resources", "diskSize")
	majorVersion, _, _ := unstructured.NestedString(u.Object, "spec", "cluster", "majorVersion")

	audit := false
	if v, found, err := unstructured.NestedBool(u.Object, "spec", "cluster", "audit", "enabled"); err == nil && found {
		audit = v
	}

	return &PostgresInstance{
		Name:              u.GetName(),
		EnvironmentName:   environmentName,
		TeamSlug:          slug.Slug(u.GetNamespace()),
		WorkloadReference: workload.ReferenceFromOwnerReferences(u.GetOwnerReferences()),
		Resources: &PostgresInstanceResources{
			CPU:      cpu,
			Memory:   memory,
			DiskSize: diskSize,
		},
		MajorVersion: majorVersion,
		Audit: PostgresInstanceAudit{
			Enabled: audit,
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

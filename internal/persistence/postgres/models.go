package postgres

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/validate"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type PostgresInstanceEdge = pagination.Edge[*PostgresInstance]

type PostgresInstanceFilter struct {
	Name         string                  `json:"name"`
	Environments []string                `json:"environments"`
	States       []PostgresInstanceState `json:"states"`
	Labels       model.LabelFilters      `json:"labels,omitempty"`
}

type PostgresInstanceConnection = pagination.FacetableConnection[*PostgresInstance, *PostgresInstanceFilter]

type PostgresInstanceFacets struct {
	AllInstances      []*PostgresInstance
	Filter            *PostgresInstanceFilter
	filteredOnce      sync.Once
	filteredInstances []*PostgresInstance
}

type PostgresInstanceStateFacetItem struct {
	State PostgresInstanceState `json:"state"`
	Count int                   `json:"count"`
}

// PostgresInstance represents an independently running database instance.
type PostgresInstance struct {
	Name            string                 `json:"name"`
	EnvironmentName string                 `json:"-"`
	TeamSlug        slug.Slug              `json:"-"`
	PostgresName    string                 `json:"postgres"`
	State           PostgresInstanceState  `json:"state"`
	Labels          []*model.ResourceLabel `json:"labels"`
}

// Postgres selects the instance that workloads use.
type Postgres struct {
	Name             string                 `json:"name"`
	EnvironmentName  string                 `json:"-"`
	TeamSlug         slug.Slug              `json:"-"`
	ActiveInstance   *string                `json:"activeInstance,omitempty"`
	MajorVersion     string                 `json:"majorVersion"`
	HighAvailability bool                   `json:"highAvailability"`
	Resources        PostgresResources      `json:"resources"`
	Labels           []*model.ResourceLabel `json:"labels"`
}

// PostgresResources contains only resource requests observed on the Postgres CR.
type PostgresResources struct {
	CPU      *string `json:"cpu,omitempty"`
	Memory   *string `json:"memory,omitempty"`
	DiskSize *string `json:"diskSize,omitempty"`
}

func (Postgres) IsNode()            {}
func (p *Postgres) ID() ident.Ident { return newPostgresIdent(p.TeamSlug, p.EnvironmentName, p.Name) }

type PostgresInstanceState string

const (
	PostgresInstanceStateAvailable   PostgresInstanceState = "AVAILABLE"
	PostgresInstanceStateProgressing PostgresInstanceState = "PROGRESSING"
	PostgresInstanceStateDegraded    PostgresInstanceState = "DEGRADED"
)

var AllPostgresInstanceState = []PostgresInstanceState{
	PostgresInstanceStateAvailable,
	PostgresInstanceStateProgressing,
	PostgresInstanceStateDegraded,
}

func (e PostgresInstanceState) IsValid() bool {
	switch e {
	case PostgresInstanceStateAvailable, PostgresInstanceStateProgressing, PostgresInstanceStateDegraded:
		return true
	}
	return false
}

func (e PostgresInstanceState) String() string {
	return string(e)
}

func (e *PostgresInstanceState) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = PostgresInstanceState(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid PostgresInstanceState", str)
	}
	return nil
}

func (e PostgresInstanceState) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

func (e *PostgresInstanceState) UnmarshalJSON(b []byte) error {
	s, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}
	return e.UnmarshalGQL(s)
}

func (e PostgresInstanceState) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	e.MarshalGQL(&buf)
	return buf.Bytes(), nil
}

func (PostgresInstance) IsPersistence() {}

func (PostgresInstance) IsNode() {}

func (PostgresInstance) IsSearchNode() {}

type DeletePostgresInput struct {
	Name            string    `json:"name"`
	EnvironmentName string    `json:"environmentName"`
	TeamSlug        slug.Slug `json:"teamSlug"`
}

func (i *DeletePostgresInput) Validate(ctx context.Context) error {
	return i.ValidationErrors(ctx).NilIfEmpty()
}

func (i *DeletePostgresInput) ValidationErrors(_ context.Context) *validate.ValidationErrors {
	verr := validate.New()
	i.Name = strings.TrimSpace(i.Name)
	i.EnvironmentName = strings.TrimSpace(i.EnvironmentName)

	if i.Name == "" {
		verr.Add("name", "Name must not be empty.")
	}
	if i.EnvironmentName == "" {
		verr.Add("environmentName", "Environment name must not be empty.")
	}
	if i.TeamSlug == "" {
		verr.Add("teamSlug", "Team slug must not be empty.")
	}

	return verr
}

type DeletePostgresPayload struct {
	PostgresDeleted *bool `json:"postgresDeleted,omitempty"`
}

// CreatePostgresAccessInput requests a new, time-limited personal database access.
// The authenticated actor and final expiry are server-controlled.
type CreatePostgresAccessInput struct {
	PostgresInstance string              `json:"postgresInstance"`
	TeamSlug         slug.Slug           `json:"teamSlug"`
	EnvironmentName  string              `json:"environmentName"`
	AccessLevel      PostgresAccessLevel `json:"accessLevel"`
	Reason           string              `json:"reason"`
	TTL              string              `json:"ttl"`
}

func (i *CreatePostgresAccessInput) Validate(ctx context.Context) error {
	return i.ValidationErrors(ctx).NilIfEmpty()
}

func (i *CreatePostgresAccessInput) ValidationErrors(ctx context.Context) *validate.ValidationErrors {
	verr := validate.New()
	i.PostgresInstance = strings.TrimSpace(i.PostgresInstance)
	i.EnvironmentName = strings.TrimSpace(i.EnvironmentName)
	i.Reason = strings.TrimSpace(i.Reason)
	i.TTL = strings.TrimSpace(i.TTL)

	if i.PostgresInstance == "" {
		verr.Add("postgresInstance", "Postgres instance must not be empty.")
	}
	if i.EnvironmentName == "" {
		verr.Add("environmentName", "Environment name must not be empty.")
	}
	if i.TeamSlug == "" {
		verr.Add("teamSlug", "Team slug must not be empty.")
	}
	if !i.AccessLevel.IsValid() {
		verr.Add("accessLevel", "Access level %q is not valid.", i.AccessLevel)
	}
	if len(i.Reason) < 10 {
		verr.Add("reason", "Reason must be at least 10 characters.")
	}
	if _, err := i.accessTTL(); err != nil {
		verr.Add("ttl", "%s", err)
	}

	if i.PostgresInstance == "" || i.EnvironmentName == "" || i.TeamSlug == "" {
		return verr
	}

	instance, err := GetReadyPostgresInstance(ctx, i.TeamSlug, i.EnvironmentName, i.PostgresInstance)
	if err != nil {
		if k8serrors.IsNotFound(err) || errors.Is(err, &watcher.ErrorNotFound{}) {
			verr.Add("postgresInstance", "Could not find PostgresInstance named %q", i.PostgresInstance)
		} else {
			verr.Add("postgresInstance", "%s", err)
		}
	} else if instance.State != PostgresInstanceStateAvailable {
		verr.Add("postgresInstance", "Postgres instance %q is not available.", i.PostgresInstance)
	}

	return verr
}

type CreatePostgresAccessPayload struct {
	Name      string    `json:"name"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type PostgresAccessLevel string

const (
	PostgresAccessLevelRead            PostgresAccessLevel = "READ"
	PostgresAccessLevelReadWrite       PostgresAccessLevel = "READWRITE"
	PostgresAccessLevelReadWriteCreate PostgresAccessLevel = "READWRITECREATE"
)

func (e PostgresAccessLevel) IsValid() bool {
	switch e {
	case PostgresAccessLevelRead, PostgresAccessLevelReadWrite, PostgresAccessLevelReadWriteCreate:
		return true
	}
	return false
}

func (e PostgresAccessLevel) String() string { return string(e) }

func (e PostgresAccessLevel) CRDValue() string { return strings.ToLower(string(e)) }

func (e *PostgresAccessLevel) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}
	*e = PostgresAccessLevel(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid PostgresAccessLevel", str)
	}
	return nil
}

func (e PostgresAccessLevel) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
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

func toPostgresInstance(u *unstructured.Unstructured, environmentName string) (*PostgresInstance, error) {
	obj := &nais_io_v1.PostgresInstance{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("converting PostgresInstance: %w", err)
	}
	if obj.Spec.Postgres == "" {
		return nil, fmt.Errorf("PostgresInstance %q has no Postgres", obj.Name)
	}
	state := PostgresInstanceStateProgressing
	if obj.Status != nil {
		state = postgresStateFromConditions(obj.Status.Conditions, obj.Status.ReconcilePhase == "Completed" && obj.Status.ObservedGeneration >= obj.Generation)
	}
	return &PostgresInstance{Name: obj.Name, EnvironmentName: environmentName, TeamSlug: slug.Slug(obj.Namespace), PostgresName: obj.Spec.Postgres, State: state, Labels: model.UserLabels(obj.Labels)}, nil
}

func toPostgres(u *unstructured.Unstructured, environmentName string) (*Postgres, error) {
	obj := &nais_io_v1.Postgres{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("converting Postgres: %w", err)
	}
	var active *string
	if obj.Status != nil && obj.Status.ActiveInstance != "" {
		active = &obj.Status.ActiveInstance
	}
	quantity := func(value resource.Quantity) *string {
		if value.IsZero() {
			return nil
		}
		text := value.String()
		return &text
	}
	return &Postgres{
		Name: obj.Name, EnvironmentName: environmentName, TeamSlug: slug.Slug(obj.Namespace),
		ActiveInstance: active, MajorVersion: obj.Spec.MajorVersion, HighAvailability: obj.Spec.HighAvailability,
		Resources: PostgresResources{
			CPU: quantity(obj.Spec.Resources.Cpu), Memory: quantity(obj.Spec.Resources.Memory),
			DiskSize: quantity(obj.Spec.Resources.DiskSize),
		},
		Labels: model.UserLabels(obj.Labels),
	}, nil
}

// postgresStateFromConditions interprets the CNPG phase mirrored by pgrator.
// ObservedState=False means no phase has been observed, not a failed cluster.
func postgresStateFromConditions(conditions []metav1.Condition, reconciled bool) PostgresInstanceState {
	if !reconciled {
		return PostgresInstanceStateProgressing
	}
	for _, condition := range conditions {
		if condition.Type != "cluster.postgresql.cnpg.io/ObservedState" || condition.Status != metav1.ConditionTrue {
			continue
		}
		phase, ok := strings.CutPrefix(condition.Message, "Cluster is in phase: ")
		if !ok {
			continue
		}
		switch phase {
		case "Cluster in healthy state":
			return PostgresInstanceStateAvailable
		case "Cluster is unrecoverable and needs manual intervention",
			"Cluster cannot proceed to reconciliation due to an unknown plugin being required",
			"Cluster cannot proceed to reconciliation due to an error while interacting with plugins",
			"Cluster has incomplete or invalid image catalog":
			return PostgresInstanceStateDegraded
		}
	}
	return PostgresInstanceStateProgressing
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

// PostgresAccess exposes the API/CLI-facing state of a controller-owned personal
// database access. Credentials are read from the controller-owned Secret on
// demand; they are never cached by the watcher.
type PostgresAccess struct {
	Name                 string              `json:"name"`
	TeamSlug             slug.Slug           `json:"-"`
	EnvironmentName      string              `json:"-"`
	PostgresInstanceName string              `json:"postgresInstance"`
	Username             string              `json:"username"`
	AccessLevel          PostgresAccessLevel `json:"accessLevel"`
	ExpiresAt            time.Time           `json:"expiresAt"`
	State                PostgresAccessState `json:"state"`
	Message              *string             `json:"message,omitempty"`
	RelayAccess          *string             `json:"relayAccess,omitempty"`
}

func (PostgresAccess) IsNode() {}

func (p *PostgresAccess) ID() ident.Ident {
	return newAccessIdent(p.TeamSlug, p.EnvironmentName, p.Name)
}

type PostgresAccessState string

const (
	PostgresAccessStatePending PostgresAccessState = "PENDING"
	PostgresAccessStateReady   PostgresAccessState = "READY"
	PostgresAccessStateFailed  PostgresAccessState = "FAILED"
	PostgresAccessStateExpired PostgresAccessState = "EXPIRED"
)

var AllPostgresAccessState = []PostgresAccessState{
	PostgresAccessStatePending,
	PostgresAccessStateReady,
	PostgresAccessStateFailed,
	PostgresAccessStateExpired,
}

func (e PostgresAccessState) IsValid() bool {
	switch e {
	case PostgresAccessStatePending, PostgresAccessStateReady, PostgresAccessStateFailed, PostgresAccessStateExpired:
		return true
	}
	return false
}

func (e PostgresAccessState) String() string {
	return string(e)
}

func (e *PostgresAccessState) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = PostgresAccessState(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid PostgresAccessState", str)
	}
	return nil
}

func (e PostgresAccessState) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

func (e *PostgresAccessState) UnmarshalJSON(b []byte) error {
	s, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}
	return e.UnmarshalGQL(s)
}

func (e PostgresAccessState) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	e.MarshalGQL(&buf)
	return buf.Bytes(), nil
}

type PostgresAccessConnectionInput struct {
	Name            string    `json:"name"`
	TeamSlug        slug.Slug `json:"teamSlug"`
	EnvironmentName string    `json:"environmentName"`
}

func (i *PostgresAccessConnectionInput) Validate(ctx context.Context) error {
	return i.ValidationErrors(ctx).NilIfEmpty()
}

func (i *PostgresAccessConnectionInput) ValidationErrors(_ context.Context) *validate.ValidationErrors {
	verr := validate.New()
	i.Name = strings.TrimSpace(i.Name)
	i.EnvironmentName = strings.TrimSpace(i.EnvironmentName)
	if i.Name == "" {
		verr.Add("name", "Name must not be empty.")
	}
	if i.TeamSlug == "" {
		verr.Add("teamSlug", "Team slug must not be empty.")
	}
	if i.EnvironmentName == "" {
		verr.Add("environmentName", "Environment name must not be empty.")
	}
	return verr
}

type PostgresAccessConnection struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	CACertificate string `json:"caCertificate"`
	ServerName    string `json:"serverName"`
	RelayEndpoint string `json:"relayEndpoint"`
	RelayAccess   string `json:"relayAccess"`
	RelayToken    string `json:"relayToken"`
}

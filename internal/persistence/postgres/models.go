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
	"github.com/nais/api/internal/workload"
	data_nais_io_v1 "github.com/nais/pgrator/pkg/api/datav1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type PostgresInstanceEdge = pagination.Edge[*PostgresInstance]

type PostgresInstanceFilter struct {
	Name             string                  `json:"name"`
	Environments     []string                `json:"environments"`
	States           []PostgresInstanceState `json:"states"`
	HighAvailability *bool                   `json:"highAvailability"`
	MajorVersions    []string                `json:"majorVersions"`
	Labels           model.LabelFilters      `json:"labels,omitempty"`
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

type PostgresInstance struct {
	Name              string                             `json:"name"`
	EnvironmentName   string                             `json:"-"`
	WorkloadReference *workload.Reference                `json:"-"`
	TeamSlug          slug.Slug                          `json:"-"`
	Resources         *PostgresInstanceResources         `json:"resources"`
	MajorVersion      string                             `json:"majorVersion"`
	Audit             PostgresInstanceAudit              `json:"audit"`
	MaintenanceWindow *PostgresInstanceMaintenanceWindow `json:"maintenanceWindow,omitempty"`
	HighAvailability  bool                               `json:"highAvailability"`
	State             PostgresInstanceState              `json:"state"`
	Labels            []*model.ResourceLabel             `json:"labels"`
}

type PostgresInstanceState string

const (
	PostgresInstanceStateAvailable   PostgresInstanceState = "AVAILABLE"
	PostgresInstanceStateProgressing PostgresInstanceState = "PROGRESSING"
	PostgresInstanceStateDegraded    PostgresInstanceState = "DEGRADED"

	postgresConditionTypeAvailable   = "Available"
	postgresConditionTypeProgressing = "Progressing"
	postgresConditionTypeDegraded    = "Degraded"
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

type PostgresInstanceAudit struct {
	Enabled          bool      `json:"enabled"`
	StatementClasses []string  `json:"statementClasses,omitempty"`
	TeamSlug         slug.Slug `json:"-"`
	EnvironmentName  string    `json:"-"`
	InstanceName     string    `json:"-"`
}

type PostgresInstanceMaintenanceWindow struct {
	Day  int `json:"day"`
	Hour int `json:"hour"`
}

func (PostgresInstance) IsPersistence() {}

func (PostgresInstance) IsNode() {}

func (PostgresInstance) IsSearchNode() {}

type PostgresInstanceResources struct {
	CPU      string `json:"cpu"`
	Memory   string `json:"memory"`
	DiskSize string `json:"diskSize"`
}

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

// CreatePostgresAccessInput requests a new, time-limited personal database access.
// The authenticated actor and access lifetime are deliberately not caller-controlled.
type CreatePostgresAccessInput struct {
	PostgresInstance         string              `json:"postgresInstance"`
	TeamSlug                 slug.Slug           `json:"teamSlug"`
	EnvironmentName          string              `json:"environmentName"`
	AccessLevel              PostgresAccessLevel `json:"accessLevel"`
	ClientWireGuardPublicKey string              `json:"clientWireGuardPublicKey"`
	Reason                   string              `json:"reason"`
}

func (i *CreatePostgresAccessInput) Validate(ctx context.Context) error {
	return i.ValidationErrors(ctx).NilIfEmpty()
}

func (i *CreatePostgresAccessInput) ValidationErrors(ctx context.Context) *validate.ValidationErrors {
	verr := validate.New()
	i.PostgresInstance = strings.TrimSpace(i.PostgresInstance)
	i.EnvironmentName = strings.TrimSpace(i.EnvironmentName)
	i.ClientWireGuardPublicKey = strings.TrimSpace(i.ClientWireGuardPublicKey)
	i.Reason = strings.TrimSpace(i.Reason)

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
	if i.ClientWireGuardPublicKey == "" {
		verr.Add("clientWireGuardPublicKey", "Client WireGuard public key must not be empty.")
	}
	if len(i.Reason) < 10 {
		verr.Add("reason", "Reason must be at least 10 characters.")
	}

	if i.PostgresInstance == "" || i.EnvironmentName == "" || i.TeamSlug == "" {
		return verr
	}

	instance, err := GetZalandoPostgres(ctx, i.TeamSlug, i.EnvironmentName, i.PostgresInstance)
	if err != nil {
		if errors.Is(err, &watcher.ErrorNotFound{}) {
			verr.Add("postgresInstance", "Could not find postgres cluster named %q", i.PostgresInstance)
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

func toPostgres(u *unstructured.Unstructured, environmentName string) (*PostgresInstance, error) {
	obj := &data_nais_io_v1.Postgres{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("converting to Postgres: %w", err)
	}

	audit := false
	statementClasses := []string(nil)
	if obj.Spec.Cluster.Audit != nil {
		audit = obj.Spec.Cluster.Audit.Enabled
		if len(obj.Spec.Cluster.Audit.StatementClasses) > 0 {
			statementClasses = make([]string, 0, len(obj.Spec.Cluster.Audit.StatementClasses))
			for _, statementClass := range obj.Spec.Cluster.Audit.StatementClasses {
				statementClasses = append(statementClasses, string(statementClass))
			}
		}
	}

	state := PostgresInstanceStateAvailable
	if obj.Status != nil {
		state = postgresStateFromConditions(obj.Status.Conditions)
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
			Enabled:          audit,
			StatementClasses: statementClasses,
			TeamSlug:         slug.Slug(obj.GetNamespace()),
			EnvironmentName:  environmentName,
			InstanceName:     obj.GetName(),
		},
		HighAvailability: obj.Spec.Cluster.HighAvailability,
		MaintenanceWindow: func() *PostgresInstanceMaintenanceWindow {
			if obj.Spec.MaintenanceWindow == nil {
				return nil
			}
			hour := 0
			if obj.Spec.MaintenanceWindow.Hour != nil {
				hour = *obj.Spec.MaintenanceWindow.Hour
			}
			return &PostgresInstanceMaintenanceWindow{
				Day:  obj.Spec.MaintenanceWindow.Day,
				Hour: hour,
			}
		}(),
		State:  state,
		Labels: model.UserLabels(obj.GetLabels()),
	}, nil
}

func postgresStateFromConditions(conditions []metav1.Condition) PostgresInstanceState {
	if meta.IsStatusConditionTrue(conditions, postgresConditionTypeDegraded) {
		return PostgresInstanceStateDegraded
	}

	if meta.IsStatusConditionTrue(conditions, postgresConditionTypeProgressing) {
		return PostgresInstanceStateProgressing
	}

	return PostgresInstanceStateAvailable
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
	Name                 string                `json:"name"`
	TeamSlug             slug.Slug             `json:"-"`
	EnvironmentName      string                `json:"-"`
	PostgresInstanceName string                `json:"postgresInstance"`
	Username             string                `json:"username"`
	AccessLevel          PostgresAccessLevel   `json:"accessLevel"`
	ExpiresAt            time.Time             `json:"expiresAt"`
	State                PostgresAccessState   `json:"state"`
	Message              *string               `json:"message,omitempty"`
	Tunnel               *PostgresAccessTunnel `json:"tunnel,omitempty"`
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

type PostgresAccessTunnel struct {
	Name             string  `json:"name"`
	Endpoint         *string `json:"endpoint,omitempty"`
	GatewayPublicKey *string `json:"gatewayPublicKey,omitempty"`
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

type PostgresAccessConnectionPayload struct {
	Password      string                         `json:"password"`
	CACertificate string                         `json:"caCertificate"`
	ServerName    string                         `json:"serverName"`
	Tunnel        PostgresAccessConnectionTunnel `json:"tunnel"`
}

type PostgresAccessConnectionTunnel struct {
	Endpoint         string `json:"endpoint"`
	GatewayPublicKey string `json:"gatewayPublicKey"`
}

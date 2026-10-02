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
	"k8s.io/apimachinery/pkg/util/validation"
)

type PostgresBranchEdge = pagination.Edge[*PostgresBranch]

type PostgresBranchFilter struct {
	Name         string                `json:"name"`
	Environments []string              `json:"environments"`
	States       []PostgresBranchState `json:"states"`
	Labels       model.LabelFilters    `json:"labels,omitempty"`
}

type PostgresBranchConnection = pagination.FacetableConnection[*PostgresBranch, *PostgresBranchFilter]

type PostgresBranchFacets struct {
	AllInstances      []*PostgresBranch
	Filter            *PostgresBranchFilter
	filteredOnce      sync.Once
	filteredInstances []*PostgresBranch
}

type PostgresBranchStateFacetItem struct {
	State PostgresBranchState `json:"state"`
	Count int                 `json:"count"`
}

// PostgresBranch represents an independently running database instance.
type PostgresBranch struct {
	Name            string                 `json:"name"`
	EnvironmentName string                 `json:"-"`
	TeamSlug        slug.Slug              `json:"-"`
	PostgresName    string                 `json:"postgres"`
	State           PostgresBranchState    `json:"state"`
	Labels          []*model.ResourceLabel `json:"labels"`
}

// Postgres selects the instance that workloads use.
type Postgres struct {
	Name             string                 `json:"name"`
	EnvironmentName  string                 `json:"-"`
	TeamSlug         slug.Slug              `json:"-"`
	ActiveBranch     *string                `json:"activeBranch,omitempty"`
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

type PostgresBranchState string

const (
	PostgresBranchStateAvailable   PostgresBranchState = "AVAILABLE"
	PostgresBranchStateProgressing PostgresBranchState = "PROGRESSING"
	PostgresBranchStateDegraded    PostgresBranchState = "DEGRADED"
)

var AllPostgresBranchState = []PostgresBranchState{
	PostgresBranchStateAvailable,
	PostgresBranchStateProgressing,
	PostgresBranchStateDegraded,
}

func (e PostgresBranchState) IsValid() bool {
	switch e {
	case PostgresBranchStateAvailable, PostgresBranchStateProgressing, PostgresBranchStateDegraded:
		return true
	}
	return false
}

func (e PostgresBranchState) String() string {
	return string(e)
}

func (e *PostgresBranchState) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = PostgresBranchState(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid PostgresBranchState", str)
	}
	return nil
}

func (e PostgresBranchState) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

func (e *PostgresBranchState) UnmarshalJSON(b []byte) error {
	s, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}
	return e.UnmarshalGQL(s)
}

func (e PostgresBranchState) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	e.MarshalGQL(&buf)
	return buf.Bytes(), nil
}

func (PostgresBranch) IsPersistence() {}

func (PostgresBranch) IsNode() {}

func (PostgresBranch) IsSearchNode() {}

type DeletePostgresBranchInput struct {
	Postgres        string    `json:"postgres"`
	Branch          string    `json:"branch"`
	EnvironmentName string    `json:"environmentName"`
	TeamSlug        slug.Slug `json:"teamSlug"`
}

func (i *DeletePostgresBranchInput) Validate(ctx context.Context) error {
	return i.ValidationErrors(ctx).NilIfEmpty()
}

func (i *DeletePostgresBranchInput) ValidationErrors(_ context.Context) *validate.ValidationErrors {
	verr := validate.New()
	i.Postgres = strings.TrimSpace(i.Postgres)
	i.Branch = strings.TrimSpace(i.Branch)
	i.EnvironmentName = strings.TrimSpace(i.EnvironmentName)

	if i.Postgres == "" {
		verr.Add("postgres", "Postgres must not be empty.")
	}
	if i.Branch == "" {
		verr.Add("branch", "Branch must not be empty.")
	}
	if i.EnvironmentName == "" {
		verr.Add("environmentName", "Environment name must not be empty.")
	}
	if i.TeamSlug == "" {
		verr.Add("teamSlug", "Team slug must not be empty.")
	}

	return verr
}

type DeletePostgresBranchPayload struct {
	PostgresBranchDeleted *bool `json:"postgresBranchDeleted,omitempty"`
}

// CreatePostgresAccessInput requests a new, time-limited personal database access.
// The authenticated actor and final expiry are server-controlled.
type CreatePostgresAccessInput struct {
	Postgres        string              `json:"postgres"`
	Branch          string              `json:"branch"`
	TeamSlug        slug.Slug           `json:"teamSlug"`
	EnvironmentName string              `json:"environmentName"`
	AccessLevel     PostgresAccessLevel `json:"accessLevel"`
	Reason          string              `json:"reason"`
	TTL             string              `json:"ttl"`
}

func (i *CreatePostgresAccessInput) Validate(ctx context.Context) error {
	return i.ValidationErrors(ctx).NilIfEmpty()
}

func (i *CreatePostgresAccessInput) ValidationErrors(ctx context.Context) *validate.ValidationErrors {
	verr := validate.New()
	i.Postgres = strings.TrimSpace(i.Postgres)
	i.Branch = strings.TrimSpace(i.Branch)
	i.EnvironmentName = strings.TrimSpace(i.EnvironmentName)
	i.Reason = strings.TrimSpace(i.Reason)
	i.TTL = strings.TrimSpace(i.TTL)

	if i.Postgres == "" {
		verr.Add("postgres", "Postgres must not be empty.")
	}
	if i.Branch == "" {
		verr.Add("branch", "Branch must not be empty.")
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

	if i.Postgres == "" || i.Branch == "" || i.EnvironmentName == "" || i.TeamSlug == "" {
		return verr
	}

	instance, err := GetReadyPostgresBranch(ctx, i.TeamSlug, i.EnvironmentName, i.Postgres, i.Branch)
	if err != nil {
		if k8serrors.IsNotFound(err) || errors.Is(err, &watcher.ErrorNotFound{}) {
			verr.Add("branch", "Could not find PostgresBranch %q in Postgres %q", i.Branch, i.Postgres)
		} else {
			verr.Add("branch", "%s", err)
		}
	} else if instance.State != PostgresBranchStateAvailable {
		verr.Add("branch", "Postgres branch %q is not available.", i.Branch)
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

func (p *PostgresBranch) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (p *PostgresBranch) DeepCopyObject() runtime.Object {
	return p
}

func (p *PostgresBranch) GetName() string {
	return nais_io_v1.PostgresBranchObjectName(p.PostgresName, p.Name)
}

func (p *PostgresBranch) SearchName() string {
	return p.PostgresName + "/" + p.Name
}

func (p *PostgresBranch) GetNamespace() string {
	return p.TeamSlug.String()
}

func (p *PostgresBranch) GetLabels() map[string]string {
	return nil
}

func (p *PostgresBranch) ID() ident.Ident {
	return newIdent(p.TeamSlug, p.EnvironmentName, p.PostgresName, p.Name)
}

func toPostgresBranch(u *unstructured.Unstructured, environmentName string) (*PostgresBranch, error) {
	obj := &nais_io_v1.PostgresBranch{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("converting PostgresBranch: %w", err)
	}
	if obj.Spec.Postgres == "" || obj.Spec.BranchName == "" || obj.Name != nais_io_v1.PostgresBranchObjectName(obj.Spec.Postgres, obj.Spec.BranchName) {
		return nil, fmt.Errorf("PostgresBranch %q has invalid postgres or branchName", obj.Name)
	}
	state := PostgresBranchStateProgressing
	if obj.Status != nil {
		state = postgresStateFromConditions(obj.Status.Conditions, obj.Status.ReconcilePhase == "Completed" && obj.Status.ObservedGeneration >= obj.Generation)
	}
	return &PostgresBranch{Name: obj.Spec.BranchName, EnvironmentName: environmentName, TeamSlug: slug.Slug(obj.Namespace), PostgresName: obj.Spec.Postgres, State: state, Labels: model.UserLabels(obj.Labels)}, nil
}

func toPostgres(u *unstructured.Unstructured, environmentName string) (*Postgres, error) {
	obj := &nais_io_v1.Postgres{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("converting Postgres: %w", err)
	}
	var active *string
	if obj.Status != nil && obj.Status.ActiveBranch != "" {
		active = &obj.Status.ActiveBranch
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
		ActiveBranch: active, MajorVersion: obj.Spec.MajorVersion, HighAvailability: obj.Spec.HighAvailability,
		Resources: PostgresResources{
			CPU: quantity(obj.Spec.Resources.Cpu), Memory: quantity(obj.Spec.Resources.Memory),
			DiskSize: quantity(obj.Spec.Resources.DiskSize),
		},
		Labels: model.UserLabels(obj.Labels),
	}, nil
}

// postgresStateFromConditions interprets the CNPG phase mirrored by pgrator.
// ObservedState=False means no phase has been observed, not a failed cluster.
func postgresStateFromConditions(conditions []metav1.Condition, reconciled bool) PostgresBranchState {
	if !reconciled {
		return PostgresBranchStateProgressing
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
			return PostgresBranchStateAvailable
		case "Cluster is unrecoverable and needs manual intervention",
			"Cluster cannot proceed to reconciliation due to an unknown plugin being required",
			"Cluster cannot proceed to reconciliation due to an error while interacting with plugins",
			"Cluster has incomplete or invalid image catalog":
			return PostgresBranchStateDegraded
		}
	}
	return PostgresBranchStateProgressing
}

type PostgresBranchOrder struct {
	Field     PostgresBranchOrderField `json:"field"`
	Direction model.OrderDirection     `json:"direction"`
}

type PostgresBranchOrderField string

const (
	PostgresBranchOrderFieldName        PostgresBranchOrderField = "NAME"
	PostgresBranchOrderFieldEnvironment PostgresBranchOrderField = "ENVIRONMENT"
)

var AllPostgresBranchOrderField = []PostgresBranchOrderField{
	PostgresBranchOrderFieldName,
	PostgresBranchOrderFieldEnvironment,
}

func (e PostgresBranchOrderField) IsValid() bool {
	switch e {
	case PostgresBranchOrderFieldName, PostgresBranchOrderFieldEnvironment:
		return true
	}
	return false
}

func (e PostgresBranchOrderField) String() string {
	return string(e)
}

func (e *PostgresBranchOrderField) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = PostgresBranchOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid PostgresBranchOrderField", str)
	}
	return nil
}

func (e PostgresBranchOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

func (e *PostgresBranchOrderField) UnmarshalJSON(b []byte) error {
	s, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}
	return e.UnmarshalGQL(s)
}

func (e PostgresBranchOrderField) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	e.MarshalGQL(&buf)
	return buf.Bytes(), nil
}

type TeamInventoryCountPostgresBranches struct {
	Total int `json:"total"`
}

// PostgresAccess exposes the API/CLI-facing state of a controller-owned personal
// database access. Credentials are read from the controller-owned Secret on
// demand; they are never cached by the watcher.
type PostgresAccess struct {
	Name               string              `json:"name"`
	TeamSlug           slug.Slug           `json:"-"`
	EnvironmentName    string              `json:"-"`
	PostgresBranchName string              `json:"postgresBranch"`
	Username           string              `json:"username"`
	AccessLevel        PostgresAccessLevel `json:"accessLevel"`
	ExpiresAt          time.Time           `json:"expiresAt"`
	State              PostgresAccessState `json:"state"`
	Message            *string             `json:"message,omitempty"`
	RelayAccess        *string             `json:"relayAccess,omitempty"`
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

type PostgresAccessConnectionDetails struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	CACertificate string `json:"caCertificate"`
	ServerName    string `json:"serverName"`
	RelayEndpoint string `json:"relayEndpoint"`
	RelayAccess   string `json:"relayAccess"`
	RelayToken    string `json:"relayToken"`
}

// supportedMajorVersions are the versions the API accepts for new Postgres. The CRD enum
// also allows older versions, but those are not offered to users.
var supportedMajorVersions = []string{"18"}

type CreatePostgresInput struct {
	Name             string    `json:"name"`
	EnvironmentName  string    `json:"environmentName"`
	TeamSlug         slug.Slug `json:"teamSlug"`
	MajorVersion     string    `json:"majorVersion"`
	HighAvailability *bool     `json:"highAvailability,omitempty"`
	CPU              *string   `json:"cpu,omitempty"`
	Memory           *string   `json:"memory,omitempty"`
	DiskSize         *string   `json:"diskSize,omitempty"`
}

type CreatePostgresPayload struct {
	Postgres *Postgres `json:"postgres"`
}

type UpdatePostgresInput struct {
	Name             string    `json:"name"`
	EnvironmentName  string    `json:"environmentName"`
	TeamSlug         slug.Slug `json:"teamSlug"`
	HighAvailability *bool     `json:"highAvailability,omitempty"`
	CPU              *string   `json:"cpu,omitempty"`
	Memory           *string   `json:"memory,omitempty"`
	DiskSize         *string   `json:"diskSize,omitempty"`
}

type UpdatePostgresPayload struct {
	Postgres *Postgres `json:"postgres"`
}

func (i *CreatePostgresInput) Validate(ctx context.Context) error {
	return i.ValidationErrors(ctx).NilIfEmpty()
}

func (i *CreatePostgresInput) ValidationErrors(_ context.Context) *validate.ValidationErrors {
	verr := validate.New()
	i.Name = strings.TrimSpace(i.Name)
	i.EnvironmentName = strings.TrimSpace(i.EnvironmentName)
	i.MajorVersion = strings.TrimSpace(i.MajorVersion)

	if i.Name == "" {
		verr.Add("name", "Name must not be empty.")
	} else if errs := validation.IsDNS1123Subdomain(i.Name); len(errs) > 0 {
		verr.Add("name", "Name must consist of lowercase letters, numbers, and hyphens only. It cannot start or end with a hyphen.")
	}
	if i.EnvironmentName == "" {
		verr.Add("environmentName", "Environment name must not be empty.")
	}
	if i.TeamSlug == "" {
		verr.Add("teamSlug", "Team slug must not be empty.")
	}
	if !contains(supportedMajorVersions, i.MajorVersion) {
		verr.Add("majorVersion", "Major version must be one of: %s.", strings.Join(supportedMajorVersions, ", "))
	}
	validateQuantities(verr, i.CPU, i.Memory, i.DiskSize)
	return verr
}

func (i *UpdatePostgresInput) Validate(ctx context.Context) error {
	return i.ValidationErrors(ctx).NilIfEmpty()
}

func (i *UpdatePostgresInput) ValidationErrors(_ context.Context) *validate.ValidationErrors {
	verr := validate.New()
	i.Name = strings.TrimSpace(i.Name)
	i.EnvironmentName = strings.TrimSpace(i.EnvironmentName)

	if i.Name == "" {
		verr.Add("name", "Name must not be empty.")
	} else if errs := validation.IsDNS1123Subdomain(i.Name); len(errs) > 0 {
		verr.Add("name", "Name must consist of lowercase letters, numbers, and hyphens only. It cannot start or end with a hyphen.")
	}
	if i.EnvironmentName == "" {
		verr.Add("environmentName", "Environment name must not be empty.")
	}
	if i.TeamSlug == "" {
		verr.Add("teamSlug", "Team slug must not be empty.")
	}
	validateQuantities(verr, i.CPU, i.Memory, i.DiskSize)
	return verr
}

func validateQuantities(verr *validate.ValidationErrors, cpu, memory, diskSize *string) {
	for field, value := range map[string]*string{"cpu": cpu, "memory": memory, "diskSize": diskSize} {
		if value == nil {
			continue
		}
		quantity, err := resource.ParseQuantity(*value)
		if err != nil {
			verr.Add(field, "%q is not a valid quantity.", *value)
		} else if quantity.Sign() <= 0 {
			verr.Add(field, "%q must be greater than zero.", *value)
		}
	}
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

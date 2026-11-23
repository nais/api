package postgres

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
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
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
)

type TeamPostgresFilter struct {
	Environments []string           `json:"environments"`
	Labels       model.LabelFilters `json:"labels,omitempty"`
}

type (
	PostgresConnection         = pagination.FacetableConnection[*Postgres, *TeamPostgresFilter]
	PostgresEdge               = pagination.Edge[*Postgres]
	WorkloadPostgresConnection = pagination.Connection[*WorkloadPostgres]
	WorkloadPostgresEdge       = pagination.Edge[*WorkloadPostgres]
	PostgresWorkloadConnection = pagination.Connection[*PostgresWorkload]
	PostgresWorkloadEdge       = pagination.Edge[*PostgresWorkload]
)

type PostgresWorkload struct {
	Workload workload.Workload `json:"workload"`
	Postgres *Postgres         `json:"postgres"`
}

type PostgresUserRole string

const (
	PostgresUserRoleAdmin     PostgresUserRole = "ADMIN"
	PostgresUserRoleRead      PostgresUserRole = "READ"
	PostgresUserRoleReadwrite PostgresUserRole = "READWRITE"
)

type WorkloadPostgres struct {
	EnvPrefix *string          `json:"envPrefix,omitempty"`
	Role      PostgresUserRole `json:"role"`
	Postgres  *Postgres        `json:"postgres"`
}

type PostgresFacets struct {
	AllInstances      []*Postgres
	FilteredInstances []*Postgres
}

// Postgres represents a database used by workloads.
type Postgres struct {
	Name                string                 `json:"name"`
	EnvironmentName     string                 `json:"-"`
	TeamSlug            slug.Slug              `json:"-"`
	MajorVersion        string                 `json:"majorVersion"`
	Extensions          []string               `json:"extensions"`
	HighAvailability    bool                   `json:"highAvailability"`
	Resources           PostgresResources      `json:"resources"`
	Labels              []*model.ResourceLabel `json:"labels"`
	CreatedAt           time.Time              `json:"createdAt"`
	Recovery            *PostgresRecovery      `json:"recovery,omitempty"`
	BackupRetentionDays int                    `json:"backupRetentionDays"`
	directClusterName   *string
	physicalState       PostgresState
	configuration       *PostgresConfiguration
}

// PostgresResources contains only resource requests observed on the Postgres CR.
type PostgresResources struct {
	CPU      *string `json:"cpu,omitempty"`
	Memory   *string `json:"memory,omitempty"`
	DiskSize *string `json:"diskSize,omitempty"`
}

func (Postgres) IsNode()            {}
func (Postgres) IsSearchNode()      {}
func (Postgres) IsActivityLogger()  {}
func (p *Postgres) ID() ident.Ident { return newPostgresIdent(p.TeamSlug, p.EnvironmentName, p.Name) }

func (p *Postgres) GetObjectKind() schema.ObjectKind { return schema.EmptyObjectKind }
func (p *Postgres) DeepCopyObject() runtime.Object   { return p }
func (p *Postgres) GetName() string                  { return p.Name }
func (p *Postgres) GetNamespace() string             { return p.TeamSlug.String() }
func (p *Postgres) GetLabels() map[string]string     { return nil }

type PostgresState string

const (
	PostgresStateAvailable   PostgresState = "AVAILABLE"
	PostgresStateProgressing PostgresState = "PROGRESSING"
	PostgresStateDegraded    PostgresState = "DEGRADED"
)

var AllPostgresState = []PostgresState{PostgresStateAvailable, PostgresStateProgressing, PostgresStateDegraded}

func (e PostgresState) IsValid() bool {
	switch e {
	case PostgresStateAvailable, PostgresStateProgressing, PostgresStateDegraded:
		return true
	}
	return false
}

func (e PostgresState) String() string { return string(e) }

func (e *PostgresState) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}
	*e = PostgresState(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid PostgresState", str)
	}
	return nil
}

func (e PostgresState) MarshalGQL(w io.Writer) { fmt.Fprint(w, strconv.Quote(e.String())) }

func (e *PostgresState) UnmarshalJSON(b []byte) error {
	s, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}
	return e.UnmarshalGQL(s)
}

func (e PostgresState) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	e.MarshalGQL(&buf)
	return buf.Bytes(), nil
}

type DeletePostgresInput struct {
	Name            string    `json:"name"`
	TeamSlug        slug.Slug `json:"teamSlug"`
	EnvironmentName string    `json:"environmentName"`
}

func (i *DeletePostgresInput) Validate(_ context.Context) error {
	verr := validate.New()
	i.Name = strings.TrimSpace(i.Name)
	i.EnvironmentName = strings.TrimSpace(i.EnvironmentName)
	if i.Name == "" || len(validation.IsDNS1123Subdomain(i.Name)) > 0 {
		verr.Add("name", "Name must be a non-empty lowercase Kubernetes DNS subdomain.")
	}
	if i.TeamSlug == "" {
		verr.Add("teamSlug", "Team slug must not be empty.")
	}
	if i.EnvironmentName == "" {
		verr.Add("environmentName", "Environment name must not be empty.")
	}
	return verr.NilIfEmpty()
}

type DeletePostgresPayload struct {
	DeletionRequested bool `json:"deletionRequested"`
}

// CreatePostgresAccessInput requests a new, time-limited personal database access.
// The authenticated actor and final expiry are server-controlled.
type CreatePostgresAccessInput struct {
	Postgres        string              `json:"postgres"`
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
	i.EnvironmentName = strings.TrimSpace(i.EnvironmentName)
	i.Reason = strings.TrimSpace(i.Reason)
	i.TTL = strings.TrimSpace(i.TTL)

	if i.Postgres == "" {
		verr.Add("postgres", "Postgres must not be empty.")
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

	if i.Postgres == "" || i.EnvironmentName == "" || i.TeamSlug == "" {
		return verr
	}

	state, err := readyPostgresState(ctx, i.TeamSlug, i.EnvironmentName, i.Postgres)
	if err != nil {
		if k8serrors.IsNotFound(err) || errors.Is(err, &watcher.ErrorNotFound{}) {
			verr.Add("postgres", "Could not find Postgres %q", i.Postgres)
		} else {
			verr.Add("postgres", "%s", err)
		}
	} else if state != PostgresStateAvailable {
		verr.Add("postgres", "Postgres %q is not available.", i.Postgres)
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
	PostgresAccessLevelAdmin           PostgresAccessLevel = "ADMIN"
	PostgresAccessLevelMaintain        PostgresAccessLevel = "MAINTAIN"
)

func (e PostgresAccessLevel) IsValid() bool {
	switch e {
	case PostgresAccessLevelRead, PostgresAccessLevelReadWrite, PostgresAccessLevelReadWriteCreate, PostgresAccessLevelAdmin, PostgresAccessLevelMaintain:
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

func toPostgres(u *unstructured.Unstructured, environmentName string) (*Postgres, error) {
	obj := &nais_io_v1.Postgres{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("converting Postgres: %w", err)
	}
	var clusterName *string
	state := PostgresStateProgressing
	var configuration *PostgresConfiguration
	if obj.Status != nil && obj.Status.ClusterName != "" {
		if len(validation.IsDNS1123Label(obj.Status.ClusterName)) > 0 {
			return nil, fmt.Errorf("Postgres %q has invalid clusterName %q", obj.Name, obj.Status.ClusterName)
		}
		clusterName = new(obj.Status.ClusterName)
		state = postgresStateFromConditions(obj.Status.Conditions, obj.Status.ReconcilePhase == "Completed" && obj.Status.ObservedGeneration >= obj.Generation)
		configuration = toPostgresConfiguration(obj.Status.Configuration)
	}
	quantity := func(value resource.Quantity) *string {
		if value.IsZero() {
			return nil
		}
		text := value.String()
		return &text
	}
	extensions := make([]string, 0, len(obj.Spec.Extensions))
	for _, extension := range obj.Spec.Extensions {
		extensions = append(extensions, extension.Name)
	}
	var recovery *PostgresRecovery
	if obj.Spec.Recovery != nil {
		recovery = &PostgresRecovery{SourcePostgres: obj.Spec.Recovery.SourcePostgres, TargetTime: obj.Spec.Recovery.TargetTime.Time}
		if obj.Status != nil {
			for _, condition := range obj.Status.Conditions {
				if condition.Type == "RecoveryCompleted" && condition.Status == metav1.ConditionTrue {
					recovery.Completed = obj.Status.Ownership != nil && obj.Status.Ownership.ClusterUID != "" && condition.Message == obj.Status.Ownership.ClusterUID
				}
				if condition.Type == "RecoveryFailed" && condition.Status == metav1.ConditionTrue {
					recovery.FailureMessage = new(condition.Message)
				}
			}
		}
	}
	return &Postgres{
		Name: obj.Name, EnvironmentName: environmentName, TeamSlug: slug.Slug(obj.Namespace),
		MajorVersion: obj.Spec.MajorVersion, Extensions: extensions, HighAvailability: obj.Spec.HighAvailability,
		Resources: PostgresResources{
			CPU: quantity(obj.Spec.Resources.Cpu), Memory: quantity(obj.Spec.Resources.Memory),
			DiskSize: quantity(obj.Spec.Resources.DiskSize),
		},
		Labels:              model.UserLabels(obj.Labels),
		BackupRetentionDays: int(obj.Spec.BackupRetentionDays()),
		CreatedAt:           obj.CreationTimestamp.Time, directClusterName: clusterName,
		physicalState: state, configuration: configuration, Recovery: recovery,
	}, nil
}

// postgresStateFromConditions interprets pgrator and CNPG status conditions.
// ObservedState=False means no phase has been observed, not a failed cluster.
func postgresStateFromConditions(conditions []metav1.Condition, reconciled bool) PostgresState {
	for _, condition := range conditions {
		if condition.Type == "RecoveryFailed" && condition.Status == metav1.ConditionTrue && condition.Reason == "Unrecoverable" {
			return PostgresStateDegraded
		}
	}
	if !reconciled {
		return PostgresStateProgressing
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
			return PostgresStateAvailable
		case "Cluster is unrecoverable and needs manual intervention",
			"Cluster cannot proceed to reconciliation due to an unknown plugin being required",
			"Cluster cannot proceed to reconciliation due to an error while interacting with plugins",
			"Cluster has incomplete or invalid image catalog":
			return PostgresStateDegraded
		}
	}
	return PostgresStateProgressing
}

// PostgresAccess exposes the API/CLI-facing state of a controller-owned personal
// database access. Credentials are read from the controller-owned Secret on
// demand; they are never cached by the watcher.
type PostgresAccess struct {
	Name            string              `json:"name"`
	TeamSlug        slug.Slug           `json:"-"`
	EnvironmentName string              `json:"-"`
	PostgresName    string              `json:"-"`
	Username        string              `json:"username"`
	AccessLevel     PostgresAccessLevel `json:"accessLevel"`
	ExpiresAt       time.Time           `json:"expiresAt"`
	State           PostgresAccessState `json:"state"`
	Message         *string             `json:"message,omitempty"`
	RelayAccess     *string             `json:"relayAccess,omitempty"`
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

type CreatePostgresInput struct {
	Name                string                 `json:"name"`
	EnvironmentName     string                 `json:"environmentName"`
	TeamSlug            slug.Slug              `json:"teamSlug"`
	MajorVersion        string                 `json:"majorVersion"`
	HighAvailability    *bool                  `json:"highAvailability,omitempty"`
	CPU                 *string                `json:"cpu,omitempty"`
	Memory              *string                `json:"memory,omitempty"`
	DiskSize            *string                `json:"diskSize,omitempty"`
	Extensions          []string               `json:"extensions,omitempty"`
	Labels              []*model.ResourceLabel `json:"labels,omitempty"`
	Recovery            *PostgresRecoveryInput `json:"recovery,omitempty"`
	BackupRetentionDays *int                   `json:"backupRetentionDays,omitempty"`
}

// PostgresRecoveryInput can only select a source in the new database's team
// and environment. Physical archive identifiers are resolved by the API.
type PostgresRecoveryInput struct {
	SourcePostgres string    `json:"sourcePostgres"`
	TargetTime     time.Time `json:"targetTime"`
}

type PostgresRecovery struct {
	SourcePostgres string    `json:"sourcePostgres"`
	TargetTime     time.Time `json:"targetTime"`
	Completed      bool      `json:"completed"`
	FailureMessage *string   `json:"failureMessage,omitempty"`
}

type CreatePostgresPayload struct {
	Postgres *Postgres `json:"postgres"`
}

type UpdatePostgresInput struct {
	Name                string                 `json:"name"`
	EnvironmentName     string                 `json:"environmentName"`
	TeamSlug            slug.Slug              `json:"teamSlug"`
	HighAvailability    *bool                  `json:"highAvailability,omitempty"`
	CPU                 *string                `json:"cpu,omitempty"`
	Memory              *string                `json:"memory,omitempty"`
	DiskSize            *string                `json:"diskSize,omitempty"`
	Extensions          []string               `json:"extensions,omitempty"`
	Labels              []*model.ResourceLabel `json:"labels,omitempty"`
	BackupRetentionDays *int                   `json:"backupRetentionDays,omitempty"`
}

type UpdatePostgresPayload struct {
	Postgres *Postgres `json:"postgres"`
}

func (i *CreatePostgresInput) Validate(ctx context.Context) error {
	if err := i.ValidationErrors(ctx).NilIfEmpty(); err != nil {
		return err
	}
	versions, err := AvailableMajorVersions(ctx, i.EnvironmentName)
	if err != nil {
		return err
	}
	if !slices.Contains(versions.Values, i.MajorVersion) {
		verr := validate.New()
		verr.Add("majorVersion", "Major version must be one of: %s.", strings.Join(versions.Values, ", "))
		return verr
	}
	return nil
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
	if i.MajorVersion == "" {
		verr.Add("majorVersion", "Major version must not be empty.")
	}
	validateQuantities(verr, i.CPU, i.Memory, i.DiskSize)
	validatePostgresUserLabels(verr, i.Labels)
	validateBackupRetentionDays(verr, i.BackupRetentionDays)
	if i.Recovery != nil {
		i.Recovery.SourcePostgres = strings.TrimSpace(i.Recovery.SourcePostgres)
		if i.Recovery.SourcePostgres == "" || len(validation.IsDNS1123Subdomain(i.Recovery.SourcePostgres)) > 0 {
			verr.Add("recovery.sourcePostgres", "Select a valid source database.")
		} else if i.Recovery.SourcePostgres == i.Name {
			verr.Add("name", "The recovered database must have a different name from its source.")
		}
		if i.Recovery.TargetTime.IsZero() || i.Recovery.TargetTime.After(time.Now()) {
			verr.Add("recovery.targetTime", "Choose an explicit restore time no later than now.")
		} else if i.Recovery.TargetTime.Nanosecond() != 0 {
			verr.Add("recovery.targetTime", "Restore time must have whole-second precision.")
		}
	}
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
	validatePostgresUserLabels(verr, i.Labels)
	validateBackupRetentionDays(verr, i.BackupRetentionDays)
	return verr
}

func validatePostgresUserLabels(verr *validate.ValidationErrors, labels []*model.ResourceLabel) {
	if err := model.ValidateUserLabels(labels); err != nil {
		verr.Add("labels", "%s", err.Error())
	}
}

func validateQuantities(verr *validate.ValidationErrors, cpu, memory, diskSize *string) {
	limits := []struct {
		field  string
		value  *string
		min    resource.Quantity
		hasMin bool
		max    resource.Quantity
	}{
		{field: "cpu", value: cpu, max: resource.MustParse("8")},
		{field: "memory", value: memory, max: resource.MustParse("32Gi")},
		{field: "diskSize", value: diskSize, min: resource.MustParse("5Gi"), hasMin: true, max: resource.MustParse("1000Gi")},
	}
	for _, limit := range limits {
		field, value := limit.field, limit.value
		if value == nil {
			continue
		}
		quantity, err := resource.ParseQuantity(*value)
		if err != nil {
			verr.Add(field, "%q is not a valid quantity.", *value)
		} else if quantity.Sign() <= 0 {
			verr.Add(field, "%q must be greater than zero.", *value)
		} else if limit.hasMin && quantity.Cmp(limit.min) < 0 {
			verr.Add(field, "%q must be at least %s.", *value, limit.min.String())
		} else if quantity.Cmp(limit.max) > 0 {
			verr.Add(field, "%q must be no greater than %s.", *value, limit.max.String())
		}
	}
}

// Mirrors the bounds of spec.backup.retentionDays in the Postgres CRD.
const (
	minBackupRetentionDays = 2
	maxBackupRetentionDays = 30
)

func validateBackupRetentionDays(verr *validate.ValidationErrors, days *int) {
	if days == nil {
		return
	}
	if *days < minBackupRetentionDays || *days > maxBackupRetentionDays {
		verr.Add("backupRetentionDays", "Backup retention must be between %d and %d days.", minBackupRetentionDays, maxBackupRetentionDays)
	}
}

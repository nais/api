package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/validate"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/dynamic"
)

// supportedMajorVersions mirrors the enum on the Postgres CRD's spec.majorVersion.
var supportedMajorVersions = []string{"16", "17", "18"}

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

func postgresGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "nais.io", Version: "v1", Resource: "postgres"}
}

// postgresClient returns a namespaced client for Postgres that writes with the API's own
// identity. Users never write the CRD directly; authorization is decided by the resolver.
func postgresClient(ctx context.Context, environmentName string, teamSlug slug.Slug) (dynamic.ResourceInterface, error) {
	client, err := fromContext(ctx).postgresBranchWatcher.SystemAuthenticatedClient(ctx, environmentName, watcher.WithImpersonatedClientGVR(postgresGVR()))
	if err != nil {
		return nil, err
	}
	return client.Namespace(teamSlug.String()), nil
}

func Create(ctx context.Context, input CreatePostgresInput) (*CreatePostgresPayload, error) {
	if err := input.Validate(ctx); err != nil {
		return nil, err
	}

	client, err := postgresClient(ctx, input.EnvironmentName, input.TeamSlug)
	if err != nil {
		return nil, err
	}

	pg := &nais_io_v1.Postgres{
		TypeMeta: metav1.TypeMeta{Kind: "Postgres", APIVersion: "nais.io/v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      input.Name,
			Namespace: input.TeamSlug.String(),
		},
		Spec: nais_io_v1.PostgresSpec{MajorVersion: input.MajorVersion},
	}
	pg.SetAnnotations(kubernetes.WithCommonAnnotations(nil, authz.ActorFromContext(ctx).User.Identity()))
	kubernetes.SetManagedByConsoleLabel(pg)
	if input.HighAvailability != nil {
		pg.Spec.HighAvailability = *input.HighAvailability
	}
	if err := applyResources(&pg.Spec.Resources, input.CPU, input.Memory, input.DiskSize); err != nil {
		return nil, err
	}

	obj, err := kubernetes.ToUnstructured(pg)
	if err != nil {
		return nil, err
	}

	created, err := client.Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return nil, apierror.ErrAlreadyExists
		}
		return nil, err
	}

	if err := logPostgresChange(ctx, activitylog.ActivityLogEntryActionCreated, input.Name, input.EnvironmentName, input.TeamSlug); err != nil {
		return nil, err
	}

	ret, err := toPostgres(created, input.EnvironmentName)
	if err != nil {
		return nil, err
	}
	return &CreatePostgresPayload{Postgres: ret}, nil
}

func Update(ctx context.Context, input UpdatePostgresInput) (*UpdatePostgresPayload, error) {
	if err := input.Validate(ctx); err != nil {
		return nil, err
	}

	client, err := postgresClient(ctx, input.EnvironmentName, input.TeamSlug)
	if err != nil {
		return nil, err
	}

	existing, err := client.Get(ctx, input.Name, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, apierror.Errorf("Postgres %q not found.", input.Name)
	}
	if err != nil {
		return nil, err
	}

	pg, err := kubernetes.ToConcrete[nais_io_v1.Postgres](existing)
	if err != nil {
		return nil, err
	}

	changed := false
	if input.HighAvailability != nil && pg.Spec.HighAvailability != *input.HighAvailability {
		pg.Spec.HighAvailability = *input.HighAvailability
		changed = true
	}
	before := pg.Spec.Resources
	if err := applyResources(&pg.Spec.Resources, input.CPU, input.Memory, input.DiskSize); err != nil {
		return nil, err
	}
	if before.Cpu.Cmp(pg.Spec.Resources.Cpu) != 0 || before.Memory.Cmp(pg.Spec.Resources.Memory) != 0 || before.DiskSize.Cmp(pg.Spec.Resources.DiskSize) != 0 {
		changed = true
	}

	if !changed {
		ret, err := toPostgres(existing, input.EnvironmentName)
		if err != nil {
			return nil, err
		}
		return &UpdatePostgresPayload{Postgres: ret}, nil
	}

	obj, err := kubernetes.ToUnstructured(pg)
	if err != nil {
		return nil, err
	}
	obj.SetAnnotations(kubernetes.WithCommonAnnotations(obj.GetAnnotations(), authz.ActorFromContext(ctx).User.Identity()))

	updated, err := client.Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	if err := logPostgresChange(ctx, activitylog.ActivityLogEntryActionUpdated, input.Name, input.EnvironmentName, input.TeamSlug); err != nil {
		return nil, err
	}

	ret, err := toPostgres(updated, input.EnvironmentName)
	if err != nil {
		return nil, err
	}
	return &UpdatePostgresPayload{Postgres: ret}, nil
}

// applyResources sets the quantities that were provided. Omitted values are left as they are,
// so the CRD defaults apply on create and existing values are kept on update.
func applyResources(r *nais_io_v1.PostgresResources, cpu, memory, diskSize *string) error {
	set := func(field string, value *string, target *resource.Quantity) error {
		if value == nil {
			return nil
		}
		q, err := resource.ParseQuantity(*value)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", field, err)
		}
		*target = q
		return nil
	}
	if err := set("cpu", cpu, &r.Cpu); err != nil {
		return err
	}
	if err := set("memory", memory, &r.Memory); err != nil {
		return err
	}
	return set("diskSize", diskSize, &r.DiskSize)
}

func logPostgresChange(ctx context.Context, action activitylog.ActivityLogEntryAction, name, environmentName string, teamSlug slug.Slug) error {
	return activitylog.Create(ctx, activitylog.CreateInput{
		Action:          action,
		Actor:           authz.ActorFromContext(ctx).User,
		ResourceType:    activityLogEntryResourceTypePostgres,
		ResourceName:    name,
		EnvironmentName: new(environmentName),
		TeamSlug:        new(teamSlug),
	})
}

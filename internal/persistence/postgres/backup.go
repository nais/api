package postgres

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	naisv1 "github.com/nais/pgrator/pkg/api/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/dynamic"
)

type PostgresBackupPhase string

const (
	PostgresBackupPhasePreparing PostgresBackupPhase = "PREPARING"
	PostgresBackupPhaseRunning   PostgresBackupPhase = "RUNNING"
	PostgresBackupPhaseCompleted PostgresBackupPhase = "COMPLETED"
	PostgresBackupPhaseFailed    PostgresBackupPhase = "FAILED"
)

type PostgresBackup struct {
	Name, SourcePostgres, EnvironmentName string
	TeamSlug                              slug.Slug
	Phase                                 PostgresBackupPhase
	RetentionDays                         int
	CreatedAt                             time.Time
	StartedAt, CompletedAt, ExpiresAt     *time.Time
	SizeBytes                             *int64
	ErrorMessage                          *string
}

func (PostgresBackup) IsNode() {}

func (p *PostgresBackup) ID() ident.Ident {
	return ident.NewIdent(identPostgresBackup, p.TeamSlug.String(), p.EnvironmentName, p.Name)
}

type (
	PostgresBackupConnection = pagination.Connection[*PostgresBackup]
	PostgresBackupEdge       = pagination.Edge[*PostgresBackup]
)

type PostgresBackupFilter struct {
	SourcePostgres *string
}

type CreatePostgresBackupInput struct {
	Name, SourcePostgres, EnvironmentName string
	TeamSlug                              slug.Slug
	RetentionDays                         *int
}

type CreatePostgresBackupPayload struct{ Backup *PostgresBackup }

type DeletePostgresBackupInput struct {
	Name, EnvironmentName string
	TeamSlug              slug.Slug
}

type DeletePostgresBackupPayload struct{ DeletionRequested bool }

func postgresBackupClient(ctx context.Context, teamSlug slug.Slug, environment string) (dynamic.ResourceInterface, error) {
	c, err := fromContext(ctx).postgresWatcher.SystemAuthenticatedClient(ctx, environment, watcher.WithImpersonatedClientGVR(naisv1.GroupVersion.WithResource("postgresbackups")))
	if err != nil {
		return nil, err
	}
	return c.Namespace(teamSlug.String()), nil
}

func GetPostgresBackupByIdent(ctx context.Context, id ident.Ident) (*PostgresBackup, error) {
	teamSlug, environment, name, err := parseAccessIdent(id)
	if err != nil {
		return nil, err
	}
	return GetPostgresBackup(ctx, teamSlug, environment, name)
}

func GetPostgresBackup(ctx context.Context, teamSlug slug.Slug, environment, name string) (*PostgresBackup, error) {
	if err := authz.CanReadPostgresBackups(ctx, teamSlug); err != nil {
		return nil, err
	}
	c, err := postgresBackupClient(ctx, teamSlug, environment)
	if err != nil {
		return nil, err
	}
	obj, err := c.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return toPostgresBackup(obj, environment)
}

func ListPostgresBackups(ctx context.Context, teamSlug slug.Slug, environment string, page *pagination.Pagination, filter *PostgresBackupFilter) (*PostgresBackupConnection, error) {
	if err := authz.CanReadPostgresBackups(ctx, teamSlug); err != nil {
		return nil, err
	}
	c, err := postgresBackupClient(ctx, teamSlug, environment)
	if err != nil {
		return nil, err
	}
	list, err := c.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return postgresBackupConnection(list.Items, environment, page, filter)
}

func postgresBackupConnection(items []unstructured.Unstructured, environment string, page *pagination.Pagination, filter *PostgresBackupFilter) (*PostgresBackupConnection, error) {
	backups := make([]*PostgresBackup, 0, len(items))
	for _, item := range items {
		backup, err := toPostgresBackup(&item, environment)
		if err != nil {
			return nil, err
		}
		if filter == nil || filter.SourcePostgres == nil || backup.SourcePostgres == *filter.SourcePostgres {
			backups = append(backups, backup)
		}
	}
	slices.SortFunc(backups, func(a, b *PostgresBackup) int {
		if n := b.CreatedAt.Compare(a.CreatedAt); n != 0 {
			return n
		}
		return cmp.Compare(a.Name, b.Name)
	})
	return pagination.NewConnection(pagination.Slice(backups, page), page, len(backups)), nil
}

func toPostgresBackup(u *unstructured.Unstructured, environment string) (*PostgresBackup, error) {
	obj, err := kubernetes.ToConcrete[naisv1.PostgresBackup](u)
	if err != nil {
		return nil, fmt.Errorf("converting PostgresBackup %q: %w", u.GetName(), err)
	}
	p := &PostgresBackup{
		Name: obj.Name, SourcePostgres: obj.Spec.Postgres, TeamSlug: slug.Slug(obj.Namespace), EnvironmentName: environment,
		CreatedAt: obj.CreationTimestamp.Time, RetentionDays: int(obj.Spec.EffectiveRetentionDays()), Phase: PostgresBackupPhasePreparing,
	}
	if s := obj.Status; s != nil {
		switch s.Phase {
		case naisv1.PostgresBackupPhaseRunning:
			p.Phase = PostgresBackupPhaseRunning
		case naisv1.PostgresBackupPhaseCompleted:
			p.Phase = PostgresBackupPhaseCompleted
		case naisv1.PostgresBackupPhaseFailed:
			p.Phase = PostgresBackupPhaseFailed
			// Controller errors can include internal paths and cloud identities.
			p.ErrorMessage = new("The backup failed. Contact support if the problem persists.")
		}
		p.StartedAt, p.CompletedAt, p.ExpiresAt = backupTime(s.StartedAt), backupTime(s.CompletedAt), backupTime(s.ExpiresAt)
		p.SizeBytes = s.SizeBytes
	}
	return p, nil
}

func backupTime(t *metav1.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t.Time
}

func validateBackupTarget(teamSlug slug.Slug, environment, name string) error {
	if teamSlug == "" || environment == "" {
		return apierror.Errorf("Team and environment are required")
	}
	if len(validation.IsDNS1123Subdomain(name)) > 0 {
		return apierror.Errorf("Backup name must be a Kubernetes DNS subdomain")
	}
	return nil
}

func (i CreatePostgresBackupInput) validate() error {
	if err := validateBackupTarget(i.TeamSlug, i.EnvironmentName, i.Name); err != nil {
		return err
	}
	if len(validation.IsDNS1123Subdomain(i.SourcePostgres)) > 0 {
		return apierror.Errorf("Source Postgres name must be a Kubernetes DNS subdomain")
	}
	if i.RetentionDays != nil && (*i.RetentionDays < 1 || *i.RetentionDays > 730) {
		return apierror.Errorf("Backup retention must be between 1 and 730 days")
	}
	return nil
}

func CreatePostgresBackup(ctx context.Context, input CreatePostgresBackupInput) (*CreatePostgresBackupPayload, error) {
	if err := authz.CanCreatePostgresBackup(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	if err := input.validate(); err != nil {
		return nil, err
	}
	pgClient, err := postgresClient(ctx, input.EnvironmentName, input.TeamSlug)
	if err != nil {
		return nil, err
	}
	source, err := pgClient.Get(ctx, input.SourcePostgres, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if source.GetDeletionTimestamp() != nil {
		return nil, apierror.Errorf("Postgres %q is being deleted", input.SourcePostgres)
	}
	c, err := postgresBackupClient(ctx, input.TeamSlug, input.EnvironmentName)
	if err != nil {
		return nil, err
	}
	backup := &naisv1.PostgresBackup{
		TypeMeta: metav1.TypeMeta{APIVersion: naisv1.GroupVersion.String(), Kind: "PostgresBackup"},
		ObjectMeta: metav1.ObjectMeta{
			Name: input.Name, Namespace: input.TeamSlug.String(),
			Annotations: kubernetes.WithCommonAnnotations(nil, authz.ActorFromContext(ctx).User.Identity()),
		},
		Spec: naisv1.PostgresBackupSpec{Postgres: input.SourcePostgres},
	}
	if input.RetentionDays != nil {
		backup.Spec.RetentionDays = new(int32(*input.RetentionDays)) // #nosec G115 -- validated to 1-730
	}
	kubernetes.SetManagedByConsoleLabel(backup)
	obj, err := kubernetes.ToUnstructured(backup)
	if err != nil {
		return nil, err
	}
	created, err := c.Create(ctx, obj, metav1.CreateOptions{FieldValidation: metav1.FieldValidationStrict})
	if k8serrors.IsAlreadyExists(err) {
		return nil, apierror.ErrAlreadyExists
	}
	if err != nil {
		return nil, err
	}
	if err := logPostgresBackupChange(ctx, activitylog.ActivityLogEntryActionCreated, input.TeamSlug, input.EnvironmentName, input.Name); err != nil {
		return nil, err
	}
	result, err := toPostgresBackup(created, input.EnvironmentName)
	if err != nil {
		return nil, err
	}
	return &CreatePostgresBackupPayload{Backup: result}, nil
}

func DeletePostgresBackup(ctx context.Context, input DeletePostgresBackupInput) (*DeletePostgresBackupPayload, error) {
	if err := authz.CanDeletePostgresBackup(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	if err := validateBackupTarget(input.TeamSlug, input.EnvironmentName, input.Name); err != nil {
		return nil, err
	}
	c, err := postgresBackupClient(ctx, input.TeamSlug, input.EnvironmentName)
	if err != nil {
		return nil, err
	}
	obj, err := c.Get(ctx, input.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if obj.GetDeletionTimestamp() != nil {
		return &DeletePostgresBackupPayload{DeletionRequested: true}, nil
	}
	uid := obj.GetUID()
	if err := c.Delete(ctx, input.Name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}}); err != nil {
		return nil, err
	}
	if err := logPostgresBackupChange(ctx, activityLogEntryActionDeletionRequested, input.TeamSlug, input.EnvironmentName, input.Name); err != nil {
		return nil, err
	}
	return &DeletePostgresBackupPayload{DeletionRequested: true}, nil
}

package elevation

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/user"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	labelElevation     = "nais.io/elevation"
	labelElevationType = "nais.io/elevation-type"

	annotationKillAfter          = "euthanaisa.nais.io/kill-after"
	annotationElevationResource  = "nais.io/elevation-resource"
	annotationElevationUser      = "nais.io/elevation-user"
	annotationElevationReason    = "nais.io/elevation-reason"
	annotationElevationCreated   = "nais.io/elevation-created"
	annotationElevationType      = "nais.io/elevation-type"
	annotationElevationNamespace = "nais.io/elevation-namespace"
)

func Create(ctx context.Context, input *CreateElevationInput, actor *authz.Actor) (*Elevation, error) {
	if err := validateInput(input); err != nil {
		return nil, err
	}

	if err := authz.CanUpdateTeamMetadata(ctx, input.Team); err != nil {
		return nil, ErrNotAuthorized
	}

	clients := fromContext(ctx)
	k8sClient, exists := clients.GetClient(input.Environment)
	if !exists {
		return nil, ErrEnvironmentNotFound
	}

	elevationID := generateElevationID()
	namespace := namespaceForTeam(input.Team)
	expiresAt := time.Now().Add(time.Duration(input.DurationMinutes) * time.Minute)
	createdAt := time.Now()

	role := buildRole(elevationID, namespace, input, actor, createdAt, expiresAt)
	_, err := k8sClient.RbacV1().Roles(namespace).Create(ctx, role, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating role: %w", err)
	}

	roleBinding := buildRoleBinding(elevationID, namespace, actor, createdAt, expiresAt)
	_, err = k8sClient.RbacV1().RoleBindings(namespace).Create(ctx, roleBinding, metav1.CreateOptions{})
	if err != nil {
		_ = k8sClient.RbacV1().Roles(namespace).Delete(ctx, elevationID, metav1.DeleteOptions{})
		return nil, fmt.Errorf("creating rolebinding: %w", err)
	}

	if err := logElevationCreated(ctx, elevationID, namespace, input, actor, expiresAt); err != nil {
		clients.log.WithError(err).Error("failed to log elevation creation")
	}

	t, err := team.Get(ctx, input.Team)
	if err != nil {
		return nil, fmt.Errorf("getting team: %w", err)
	}

	u, err := user.Get(ctx, actor.User.GetID())
	if err != nil {
		return nil, fmt.Errorf("getting user: %w", err)
	}

	return &Elevation{
		ID:           newIdent(elevationID),
		Type:         input.Type,
		Team:         t,
		Environment:  input.Environment,
		ResourceName: input.ResourceName,
		User:         u,
		Reason:       input.Reason,
		CreatedAt:    createdAt,
		ExpiresAt:    expiresAt,
	}, nil
}

func validateInput(input *CreateElevationInput) error {
	if !input.Type.IsValid() {
		return fmt.Errorf("invalid elevation type: %s", input.Type)
	}

	if len(input.Reason) < 10 {
		return ErrReasonTooShort
	}

	if input.DurationMinutes < 1 || input.DurationMinutes > 60 {
		return ErrInvalidDuration
	}

	return nil
}

func generateElevationID() string {
	return fmt.Sprintf("elev-%s", uuid.New().String()[:8])
}

func namespaceForTeam(teamSlug slug.Slug) string {
	return teamSlug.String()
}

func buildRole(elevationID, namespace string, input *CreateElevationInput, actor *authz.Actor, createdAt, expiresAt time.Time) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      elevationID,
			Namespace: namespace,
			Labels: map[string]string{
				labelElevation:     "true",
				labelElevationType: string(input.Type),
			},
			Annotations: map[string]string{
				annotationKillAfter:          expiresAt.Format(time.RFC3339),
				annotationElevationResource:  input.ResourceName,
				annotationElevationUser:      actor.User.Identity(),
				annotationElevationReason:    input.Reason,
				annotationElevationCreated:   createdAt.Format(time.RFC3339),
				annotationElevationType:      string(input.Type),
				annotationElevationNamespace: namespace,
			},
		},
		Rules: getRoleRules(input.Type, input.ResourceName),
	}
}

func buildRoleBinding(elevationID, namespace string, actor *authz.Actor, createdAt, expiresAt time.Time) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      elevationID,
			Namespace: namespace,
			Labels: map[string]string{
				labelElevation: "true",
			},
			Annotations: map[string]string{
				annotationKillAfter:        expiresAt.Format(time.RFC3339),
				annotationElevationCreated: createdAt.Format(time.RFC3339),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     elevationID,
		},
		Subjects: []rbacv1.Subject{{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "User",
			Name:     actor.User.Identity(),
		}},
	}
}

func getRoleRules(elevationType ElevationType, resourceName string) []rbacv1.PolicyRule {
	var rule rbacv1.PolicyRule

	switch elevationType {
	case ElevationTypeSecret:
		rule = rbacv1.PolicyRule{
			APIGroups:     []string{""},
			Resources:     []string{"secrets"},
			Verbs:         []string{"get"},
			ResourceNames: []string{resourceName},
		}
	case ElevationTypePodExec:
		rule = rbacv1.PolicyRule{
			APIGroups:     []string{""},
			Resources:     []string{"pods/exec"},
			Verbs:         []string{"create"},
			ResourceNames: []string{resourceName},
		}
	case ElevationTypePodPortForward:
		rule = rbacv1.PolicyRule{
			APIGroups:     []string{""},
			Resources:     []string{"pods/portforward"},
			Verbs:         []string{"create"},
			ResourceNames: []string{resourceName},
		}
	case ElevationTypePodDebug:
		rule = rbacv1.PolicyRule{
			APIGroups:     []string{""},
			Resources:     []string{"pods/ephemeralcontainers"},
			Verbs:         []string{"patch"},
			ResourceNames: []string{resourceName},
		}
	}

	return []rbacv1.PolicyRule{rule}
}

func logElevationCreated(ctx context.Context, elevationID, namespace string, input *CreateElevationInput, actor *authz.Actor, expiresAt time.Time) error {
	return database.Transaction(ctx, func(ctx context.Context) error {
		return activitylog.Create(ctx, activitylog.CreateInput{
			Actor:           actor.User,
			Action:          activitylog.ActivityLogEntryActionCreated,
			ResourceType:    activityLogEntryResourceTypeElevation,
			ResourceName:    elevationID,
			TeamSlug:        &input.Team,
			EnvironmentName: &input.Environment,
			Data: &ElevationCreatedActivityLogEntryData{
				ElevationType:      string(input.Type),
				TargetResourceName: input.ResourceName,
				Reason:             input.Reason,
				ExpiresAt:          expiresAt.Format(time.RFC3339),
			},
		})
	})
}

// List returns active elevations for the current user by type, team, environment and resourceName
func List(ctx context.Context, input *ElevationInput, actor *authz.Actor) ([]*Elevation, error) {
	clients := fromContext(ctx)

	k8sClient, exists := clients.GetClient(input.Environment)
	if !exists {
		return []*Elevation{}, nil // Environment not found, return empty list
	}

	namespace := namespaceForTeam(input.Team)
	userIdentity := actor.User.Identity()

	roles, err := k8sClient.RbacV1().Roles(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=true,%s=%s", labelElevation, labelElevationType, string(input.Type)),
	})
	if err != nil {
		return nil, fmt.Errorf("listing roles: %w", err)
	}

	var elevations []*Elevation
	for _, role := range roles.Items {
		// Filter by user
		if role.Annotations[annotationElevationUser] != userIdentity {
			continue
		}
		// Filter by resourceName
		if role.Annotations[annotationElevationResource] != input.ResourceName {
			continue
		}

		elev, err := roleToElevation(ctx, &role, input.Environment)
		if err != nil {
			clients.log.WithError(err).WithField("role", role.Name).Debug("failed to convert role to elevation")
			continue
		}
		elevations = append(elevations, elev)
	}

	return elevations, nil
}

// roleToElevation converts a Kubernetes Role to an Elevation
func roleToElevation(ctx context.Context, role *rbacv1.Role, environment string) (*Elevation, error) {
	elevationType := ElevationType(role.Annotations[annotationElevationType])
	if !elevationType.IsValid() {
		elevationType = ElevationType(role.Labels[labelElevationType])
	}

	createdAtStr := role.Annotations[annotationElevationCreated]
	createdAt, err := time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		createdAt = role.CreationTimestamp.Time
	}

	expiresAtStr := role.Annotations[annotationKillAfter]
	expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
	if err != nil {
		expiresAt = createdAt.Add(time.Hour)
	}

	teamSlug := slug.Slug(role.Namespace)
	t, err := team.Get(ctx, teamSlug)
	if err != nil {
		return nil, fmt.Errorf("getting team: %w", err)
	}

	userIdentity := role.Annotations[annotationElevationUser]
	u, err := user.GetByEmail(ctx, userIdentity)
	if err != nil {
		return nil, fmt.Errorf("getting user: %w", err)
	}

	return &Elevation{
		ID:           newIdent(role.Name),
		Type:         elevationType,
		Team:         t,
		Environment:  environment,
		ResourceName: role.Annotations[annotationElevationResource],
		User:         u,
		Reason:       role.Annotations[annotationElevationReason],
		CreatedAt:    createdAt,
		ExpiresAt:    expiresAt,
	}, nil
}

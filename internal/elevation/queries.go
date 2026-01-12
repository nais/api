package elevation

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/ident"
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
	// Validate input
	if err := validateInput(input); err != nil {
		return nil, err
	}

	// Check team authorization - user must be team member or owner
	if err := authz.CanUpdateTeamMetadata(ctx, input.Team); err != nil {
		return nil, ErrNotAuthorized
	}

	// Get Kubernetes client for environment
	clients := fromContext(ctx)
	k8sClient, exists := clients.GetClient(input.Environment)
	if !exists {
		return nil, ErrEnvironmentNotFound
	}

	// Generate unique ID for Role/RoleBinding
	elevationID := generateElevationID()
	namespace := namespaceForTeam(input.Team)
	expiresAt := time.Now().Add(time.Duration(input.DurationMinutes) * time.Minute)
	createdAt := time.Now()

	// Create Role
	role := buildRole(elevationID, namespace, input, actor, createdAt, expiresAt)
	_, err := k8sClient.RbacV1().Roles(namespace).Create(ctx, role, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating role: %w", err)
	}

	// Create RoleBinding
	roleBinding := buildRoleBinding(elevationID, namespace, actor, createdAt, expiresAt)
	_, err = k8sClient.RbacV1().RoleBindings(namespace).Create(ctx, roleBinding, metav1.CreateOptions{})
	if err != nil {
		// Clean up role if rolebinding creation fails
		_ = k8sClient.RbacV1().Roles(namespace).Delete(ctx, elevationID, metav1.DeleteOptions{})
		return nil, fmt.Errorf("creating rolebinding: %w", err)
	}

	// Log to activity log
	if err := logElevationCreated(ctx, elevationID, namespace, input, actor, expiresAt); err != nil {
		clients.log.WithError(err).Error("failed to log elevation creation")
	}

	// Build response
	t, err := team.Get(ctx, input.Team)
	if err != nil {
		return nil, fmt.Errorf("getting team: %w", err)
	}

	u, err := user.Get(ctx, actor.User.GetID())
	if err != nil {
		return nil, fmt.Errorf("getting user: %w", err)
	}

	return &Elevation{
		ID:           ident.NewIdent("Elevation", elevationID),
		Type:         input.Type,
		Team:         t,
		Environment:  input.Environment,
		ResourceName: input.ResourceName,
		User:         u,
		Reason:       input.Reason,
		CreatedAt:    createdAt,
		ExpiresAt:    expiresAt,
		Status:       ElevationStatusActive,
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

func Revoke(ctx context.Context, input *RevokeElevationInput, actor *authz.Actor) error {
	// Parse elevation ID to get the resource name
	elevationID := input.ElevationID.ID

	// We need to find the elevation first to get the namespace/environment
	// Try all environments to find the Role
	clients := fromContext(ctx)

	for environment, k8sClient := range clients.k8sClients {
		// Try to get the Role in each environment
		role, err := k8sClient.RbacV1().Roles("").List(ctx, metav1.ListOptions{
			LabelSelector: labelElevation + "=true",
			FieldSelector: "metadata.name=" + elevationID,
		})
		if err != nil {
			clients.log.WithError(err).WithField("environment", environment).Debug("failed to list roles")
			continue
		}

		if len(role.Items) == 0 {
			continue
		}

		foundRole := &role.Items[0]
		namespace := foundRole.Namespace

		// Check authorization - user must be the owner of the elevation or team owner
		elevationUser := foundRole.Annotations[annotationElevationUser]
		if elevationUser != actor.User.Identity() {
			// Not the owner, check if user is team owner
			teamSlug := slug.Slug(namespace)
			if err := authz.CanManageTeamMembers(ctx, teamSlug); err != nil {
				return ErrNotAuthorized
			}
		}

		// Delete RoleBinding first
		err = k8sClient.RbacV1().RoleBindings(namespace).Delete(ctx, elevationID, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("deleting rolebinding: %w", err)
		}

		// Delete Role
		err = k8sClient.RbacV1().Roles(namespace).Delete(ctx, elevationID, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("deleting role: %w", err)
		}

		// Log to activity log
		teamSlug := slug.Slug(namespace)
		if err := logElevationRevoked(ctx, elevationID, namespace, &teamSlug, &environment, actor); err != nil {
			clients.log.WithError(err).Error("failed to log elevation revocation")
		}

		return nil
	}

	return ErrElevationNotFound
}

func logElevationRevoked(ctx context.Context, elevationID, namespace string, teamSlug *slug.Slug, environment *string, actor *authz.Actor) error {
	return database.Transaction(ctx, func(ctx context.Context) error {
		return activitylog.Create(ctx, activitylog.CreateInput{
			Actor:           actor.User,
			Action:          activityLogEntryActionRevoked,
			ResourceType:    activityLogEntryResourceTypeElevation,
			ResourceName:    elevationID,
			TeamSlug:        teamSlug,
			EnvironmentName: environment,
		})
	})
}

// Get returns a single elevation by ID
func Get(ctx context.Context, elevationID string) (*Elevation, error) {
	clients := fromContext(ctx)

	for environment, k8sClient := range clients.k8sClients {
		role, err := k8sClient.RbacV1().Roles("").List(ctx, metav1.ListOptions{
			LabelSelector: labelElevation + "=true",
			FieldSelector: "metadata.name=" + elevationID,
		})
		if err != nil {
			clients.log.WithError(err).WithField("environment", environment).Debug("failed to list roles")
			continue
		}

		if len(role.Items) == 0 {
			continue
		}

		return roleToElevation(ctx, &role.Items[0], environment)
	}

	return nil, ErrElevationNotFound
}

// ListForUser returns all active elevations for the current user across all environments
func ListForUser(ctx context.Context, actor *authz.Actor) ([]*Elevation, error) {
	clients := fromContext(ctx)
	userIdentity := actor.User.Identity()

	var elevations []*Elevation

	for environment, k8sClient := range clients.k8sClients {
		roles, err := k8sClient.RbacV1().Roles("").List(ctx, metav1.ListOptions{
			LabelSelector: labelElevation + "=true",
		})
		if err != nil {
			clients.log.WithError(err).WithField("environment", environment).Debug("failed to list roles")
			continue
		}

		for _, role := range roles.Items {
			// Filter by user
			if role.Annotations[annotationElevationUser] != userIdentity {
				continue
			}

			elev, err := roleToElevation(ctx, &role, environment)
			if err != nil {
				clients.log.WithError(err).WithField("role", role.Name).Debug("failed to convert role to elevation")
				continue
			}

			elevations = append(elevations, elev)
		}
	}

	return elevations, nil
}

// ListForTeamEnvironment returns all active elevations for a specific team and environment
func ListForTeamEnvironment(ctx context.Context, teamSlug slug.Slug, environment string) ([]*Elevation, error) {
	clients := fromContext(ctx)

	k8sClient, exists := clients.GetClient(environment)
	if !exists {
		return nil, ErrEnvironmentNotFound
	}

	namespace := namespaceForTeam(teamSlug)

	roles, err := k8sClient.RbacV1().Roles(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelElevation + "=true",
	})
	if err != nil {
		return nil, fmt.Errorf("listing roles: %w", err)
	}

	var elevations []*Elevation
	for _, role := range roles.Items {
		elev, err := roleToElevation(ctx, &role, environment)
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
		// Default to 1 hour from creation if not set
		expiresAt = createdAt.Add(time.Hour)
	}

	// Determine status
	status := ElevationStatusActive
	if time.Now().After(expiresAt) {
		status = ElevationStatusExpired
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
		ID:           ident.NewIdent("Elevation", role.Name),
		Type:         elevationType,
		Team:         t,
		Environment:  environment,
		ResourceName: role.Annotations[annotationElevationResource],
		User:         u,
		Reason:       role.Annotations[annotationElevationReason],
		CreatedAt:    createdAt,
		ExpiresAt:    expiresAt,
		Status:       status,
	}, nil
}

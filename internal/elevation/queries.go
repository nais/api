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
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	labelElevation         = "nais.io/elevation"
	labelElevationType     = "nais.io/elevation-type"
	labelEuthanaisaEnabled = "euthanaisa.nais.io/enabled"

	annotationKillAfter          = "euthanaisa.nais.io/kill-after"
	annotationElevationResource  = "nais.io/elevation-resource"
	annotationElevationUser      = "nais.io/elevation-user"
	annotationElevationReason    = "nais.io/elevation-reason"
	annotationElevationCreated   = "nais.io/elevation-created"
	annotationElevationType      = "nais.io/elevation-type"
	annotationElevationNamespace = "nais.io/elevation-namespace"
)

var (
	roleGVR        = schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"}
	roleBindingGVR = schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"}
)

func Create(ctx context.Context, input *CreateElevationInput, actor *authz.Actor) (*Elevation, error) {
	if err := validateInput(input); err != nil {
		return nil, err
	}

	if err := authz.CanUpdateTeamMetadata(ctx, input.Team); err != nil {
		return nil, ErrNotAuthorized
	}

	clients := fromContext(ctx)
	k8sClient, exists := clients.GetClient(input.EnvironmentName)
	if !exists {
		return nil, ErrEnvironmentNotFound
	}

	elevationID := generateElevationID()
	namespace := input.Team.String()
	expiresAt := time.Now().Add(time.Duration(input.DurationMinutes) * time.Minute)
	createdAt := time.Now()

	role := buildRoleUnstructured(elevationID, namespace, input, actor, createdAt, expiresAt)
	_, err := k8sClient.Resource(roleGVR).Namespace(namespace).Create(ctx, role, metav1.CreateOptions{})
	if err != nil {
		clients.log.WithError(err).WithField("namespace", namespace).Error("failed to create elevation role")
		return nil, fmt.Errorf("creating role: %w", err)
	}

	roleBinding := buildRoleBindingUnstructured(elevationID, namespace, actor, createdAt, expiresAt)
	_, err = k8sClient.Resource(roleBindingGVR).Namespace(namespace).Create(ctx, roleBinding, metav1.CreateOptions{})
	if err != nil {
		clients.log.WithError(err).WithField("namespace", namespace).Error("failed to create elevation rolebinding")
		_ = k8sClient.Resource(roleGVR).Namespace(namespace).Delete(ctx, elevationID, metav1.DeleteOptions{})
		return nil, fmt.Errorf("creating rolebinding: %w", err)
	}

	if err := logElevationCreated(ctx, elevationID, namespace, input, actor, expiresAt); err != nil {
		clients.log.WithError(err).Error("failed to log elevation creation")
	}

	return &Elevation{
		ID:              newIdent(input.Team, input.EnvironmentName, elevationID),
		Type:            input.Type,
		TeamSlug:        input.Team,
		EnvironmentName: input.EnvironmentName,
		ResourceName:    input.ResourceName,
		UserEmail:       actor.User.Identity(),
		Reason:          input.Reason,
		CreatedAt:       createdAt,
		ExpiresAt:       expiresAt,
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

func buildRoleUnstructured(elevationID, namespace string, input *CreateElevationInput, actor *authz.Actor, createdAt, expiresAt time.Time) *unstructured.Unstructured {
	rules := getRoleRules(input.Type, input.ResourceName)
	rulesUnstructured := make([]any, len(rules))
	for i, rule := range rules {
		// Convert string slices to []any for proper unstructured serialization
		apiGroups := make([]any, len(rule.APIGroups))
		for j, v := range rule.APIGroups {
			apiGroups[j] = v
		}
		resources := make([]any, len(rule.Resources))
		for j, v := range rule.Resources {
			resources[j] = v
		}
		verbs := make([]any, len(rule.Verbs))
		for j, v := range rule.Verbs {
			verbs[j] = v
		}
		resourceNames := make([]any, len(rule.ResourceNames))
		for j, v := range rule.ResourceNames {
			resourceNames[j] = v
		}
		rulesUnstructured[i] = map[string]any{
			"apiGroups":     apiGroups,
			"resources":     resources,
			"verbs":         verbs,
			"resourceNames": resourceNames,
		}
	}

	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "Role",
			"metadata": map[string]any{
				"name":      elevationID,
				"namespace": namespace,
				"labels": map[string]any{
					labelElevation:         "true",
					labelElevationType:     string(input.Type),
					labelEuthanaisaEnabled: "true",
				},
				"annotations": map[string]any{
					annotationKillAfter:          expiresAt.Format(time.RFC3339),
					annotationElevationResource:  input.ResourceName,
					annotationElevationUser:      actor.User.Identity(),
					annotationElevationReason:    input.Reason,
					annotationElevationCreated:   createdAt.Format(time.RFC3339),
					annotationElevationType:      string(input.Type),
					annotationElevationNamespace: namespace,
				},
			},
			"rules": rulesUnstructured,
		},
	}
}

func buildRoleBindingUnstructured(elevationID, namespace string, actor *authz.Actor, createdAt, expiresAt time.Time) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "RoleBinding",
			"metadata": map[string]any{
				"name":      elevationID,
				"namespace": namespace,
				"labels": map[string]any{
					labelElevation:         "true",
					labelEuthanaisaEnabled: "true",
				},
				"annotations": map[string]any{
					annotationKillAfter:        expiresAt.Format(time.RFC3339),
					annotationElevationCreated: createdAt.Format(time.RFC3339),
				},
			},
			"roleRef": map[string]any{
				"apiGroup": "rbac.authorization.k8s.io",
				"kind":     "Role",
				"name":     elevationID,
			},
			"subjects": []any{
				map[string]any{
					"apiGroup": "rbac.authorization.k8s.io",
					"kind":     "User",
					"name":     actor.User.Identity(),
				},
			},
		},
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
	case ElevationTypeExec:
		rule = rbacv1.PolicyRule{
			APIGroups:     []string{""},
			Resources:     []string{"pods/exec"},
			Verbs:         []string{"create"},
			ResourceNames: []string{resourceName},
		}
	case ElevationTypePortForward:
		rule = rbacv1.PolicyRule{
			APIGroups:     []string{""},
			Resources:     []string{"pods/portforward"},
			Verbs:         []string{"create"},
			ResourceNames: []string{resourceName},
		}
	case ElevationTypeDebug:
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
			EnvironmentName: &input.EnvironmentName,
			Data: &ElevationCreatedActivityLogEntryData{
				ElevationType:      input.Type,
				TargetResourceName: input.ResourceName,
				Reason:             input.Reason,
				ExpiresAt:          expiresAt,
			},
		})
	})
}

// Get returns a specific elevation by team, environment and elevation ID
func Get(ctx context.Context, teamSlug slug.Slug, environmentName, elevationID string) (*Elevation, error) {
	clients := fromContext(ctx)

	k8sClient, exists := clients.GetClient(environmentName)
	if !exists {
		return nil, ErrEnvironmentNotFound
	}

	namespace := teamSlug.String()

	role, err := k8sClient.Resource(roleGVR).Namespace(namespace).Get(ctx, elevationID, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting role: %w", err)
	}

	// Verify this is actually an elevation role
	labels := role.GetLabels()
	if labels[labelElevation] != "true" {
		return nil, fmt.Errorf("role is not an elevation")
	}

	return unstructuredToElevation(role, environmentName)
}

// List returns active elevations for the specified user by type, team, environment and resourceName
func List(ctx context.Context, input *ElevationInput, userEmail string) ([]*Elevation, error) {
	clients := fromContext(ctx)

	k8sClient, exists := clients.GetClient(input.EnvironmentName)
	if !exists {
		return []*Elevation{}, nil // Environment not found, return empty list
	}

	namespace := input.Team.String()

	roles, err := k8sClient.Resource(roleGVR).Namespace(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=true,%s=%s", labelElevation, labelElevationType, string(input.Type)),
	})
	if err != nil {
		return nil, fmt.Errorf("listing roles: %w", err)
	}

	var elevations []*Elevation
	for _, role := range roles.Items {
		annotations := role.GetAnnotations()
		// Filter by user
		if annotations[annotationElevationUser] != userEmail {
			continue
		}
		// Filter by resourceName
		if annotations[annotationElevationResource] != input.ResourceName {
			continue
		}

		elev, err := unstructuredToElevation(&role, input.EnvironmentName)
		if err != nil {
			clients.log.WithError(err).WithField("role", role.GetName()).Debug("failed to convert role to elevation")
			continue
		}
		elevations = append(elevations, elev)
	}

	return elevations, nil
}

// unstructuredToElevation converts an unstructured Role to an Elevation
func unstructuredToElevation(role *unstructured.Unstructured, environmentName string) (*Elevation, error) {
	annotations := role.GetAnnotations()
	labels := role.GetLabels()

	elevationType := ElevationType(annotations[annotationElevationType])
	if !elevationType.IsValid() {
		elevationType = ElevationType(labels[labelElevationType])
	}

	createdAtStr := annotations[annotationElevationCreated]
	createdAt, err := time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		createdAt = role.GetCreationTimestamp().Time
	}

	expiresAtStr := annotations[annotationKillAfter]
	expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
	if err != nil {
		expiresAt = createdAt.Add(time.Hour)
	}

	teamSlug := slug.Slug(role.GetNamespace())

	return &Elevation{
		ID:              newIdent(teamSlug, environmentName, role.GetName()),
		Type:            elevationType,
		TeamSlug:        teamSlug,
		EnvironmentName: environmentName,
		ResourceName:    annotations[annotationElevationResource],
		UserEmail:       annotations[annotationElevationUser],
		Reason:          annotations[annotationElevationReason],
		CreatedAt:       createdAt,
		ExpiresAt:       expiresAt,
	}, nil
}

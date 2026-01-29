// Package secret provides operations for managing Kubernetes secrets.
//
// Secret operations use different Kubernetes clients based on security requirements:
//
// SystemAuthenticatedClient (nais-api service account):
//   - List/Get secret metadata and keys
//   - Create secrets
//   - Add/Update/Remove secret values (using JSON Patch - never reads values)
//   - Delete secrets
//   - Allows admin bypass - admins can manage secrets in any team
//
// ImpersonatedClient (user's RBAC):
//   - Read secret values
//   - Requires user to be team member AND have active elevation
//   - NO admin bypass - even admins must be team members and request elevation
//   - This ensures all secret value access is audited via elevation system
package secret

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"
)

func ListForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName string, w workload.Workload, page *pagination.Pagination) (*SecretConnection, error) {
	secretNames := w.GetSecrets()
	allSecrets := watcher.Objects(fromContext(ctx).Watcher().GetByNamespace(teamSlug.String(), watcher.InCluster(environmentName)))

	ret := make([]*Secret, 0, len(allSecrets))
	for _, s := range allSecrets {
		if slices.Contains(secretNames, s.Name) {
			ret = append(ret, s)
		}
	}

	SortFilter.Sort(ctx, ret, "NAME", model.OrderDirectionAsc)
	paginated := pagination.Slice(ret, page)
	return pagination.NewConnection(paginated, page, len(ret)), nil
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *SecretOrder, filter *SecretFilter) (*SecretConnection, error) {
	allSecrets := watcher.Objects(fromContext(ctx).Watcher().GetByNamespace(teamSlug.String()))

	if orderBy == nil {
		orderBy = &SecretOrder{
			Field:     "NAME",
			Direction: model.OrderDirectionAsc,
		}
	}

	filtered := SortFilter.Filter(ctx, allSecrets, filter)
	SortFilter.Sort(ctx, filtered, orderBy.Field, orderBy.Direction)

	secrets := pagination.Slice(filtered, page)
	return pagination.NewConnection(secrets, page, len(filtered)), nil
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*Secret, error) {
	secret, err := fromContext(ctx).Watcher().Get(environment, teamSlug.String(), name)
	if err != nil {
		return nil, err
	}
	return secret, nil
}

func GetByIdent(ctx context.Context, id ident.Ident) (*Secret, error) {
	teamSlug, env, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, teamSlug, env, name)
}

// GetSecretKeys returns only the key names from a secret using the cached data.
// This does not require elevation as it does not return the actual values.
// The keys are stored in an annotation by the transformer during caching.
func GetSecretKeys(ctx context.Context, teamSlug slug.Slug, environmentName, name string) ([]string, error) {
	secret, err := fromContext(ctx).Watcher().Get(environmentName, teamSlug.String(), name)
	if err != nil {
		return nil, err
	}

	return secret.Keys, nil
}

// GetSecretValues returns the secret values using an impersonated client.
// This requires elevation as it returns the actual secret values.
func GetSecretValues(ctx context.Context, teamSlug slug.Slug, environmentName, name string) ([]*SecretValue, error) {
	client, err := fromContext(ctx).Client(ctx, environmentName)
	if err != nil {
		return nil, err
	}

	u, err := client.Namespace(teamSlug.String()).Get(ctx, name, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	data, _, err := unstructured.NestedStringMap(u.Object, "data")
	if err != nil {
		return nil, err
	}

	vars := make([]*SecretValue, 0, len(data))
	for k, v := range data {
		val, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, err
		}

		vars = append(vars, &SecretValue{
			Name:  k,
			Value: string(val),
		})
	}

	slices.SortFunc(vars, func(a, b *SecretValue) int {
		return strings.Compare(a.Name, b.Name)
	})

	return vars, nil
}

func Create(ctx context.Context, teamSlug slug.Slug, environment, name string) (*Secret, error) {
	w := fromContext(ctx).Watcher()
	client, err := w.SystemAuthenticatedClient(ctx, environment)
	if err != nil {
		return nil, err
	}

	if nameErrs := validation.IsDNS1123Subdomain(name); len(nameErrs) > 0 {
		return nil, fmt.Errorf("invalid name %q: %s", name, strings.Join(nameErrs, ", "))
	}

	actor := authz.ActorFromContext(ctx)

	secret := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:        name,
			Namespace:   teamSlug.String(),
			Annotations: annotations(actor.User.Identity()),
		},
		Type: corev1.SecretTypeOpaque,
	}

	kubernetes.SetManagedByConsoleLabel(secret)

	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(secret)
	if err != nil {
		return nil, err
	}

	s, err := client.Namespace(teamSlug.String()).Create(ctx, &unstructured.Unstructured{Object: u}, v1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return nil, ErrAlreadyExists
		}
		return nil, fmt.Errorf("creating secret: %w", err)
	}

	err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activitylog.ActivityLogEntryActionCreated,
		Actor:           actor.User,
		EnvironmentName: ptr.To(environment),
		ResourceType:    activityLogEntryResourceTypeSecret,
		ResourceName:    name,
		TeamSlug:        ptr.To(teamSlug),
	})
	if err != nil {
		fromContext(ctx).log.WithError(err).Errorf("unable to create activity log entry")
	}

	retVal, ok := toGraphSecret(s, environment)
	if !ok {
		return nil, fmt.Errorf("failed to convert secret to graph secret")
	}
	return retVal, nil
}

func AddSecretValue(ctx context.Context, teamSlug slug.Slug, environment, secretName string, valueToAdd *SecretValueInput) (*Secret, error) {
	if err := validateSecretValue(valueToAdd); err != nil {
		return nil, err
	}

	w := fromContext(ctx).Watcher()
	client, err := w.SystemAuthenticatedClient(ctx, environment)
	if err != nil {
		return nil, err
	}

	// First check if the secret exists and is managed by console (without reading values)
	obj, err := client.Namespace(teamSlug.String()).Get(ctx, secretName, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if !secretIsManagedByConsole(obj) {
		return nil, ErrUnmanaged
	}

	// Check if key already exists by looking at data keys (not values)
	data, dataExists, _ := unstructured.NestedMap(obj.Object, "data")
	if _, exists := data[valueToAdd.Name]; exists {
		return nil, apierror.Errorf("The secret already contains a secret value with the name %q.", valueToAdd.Name)
	}

	// Use JSON Patch to add the new key without reading existing values
	actor := authz.ActorFromContext(ctx)
	encodedValue := base64.StdEncoding.EncodeToString([]byte(valueToAdd.Value))

	var patch []map[string]any
	if !dataExists || data == nil {
		// If /data doesn't exist, we need to create it first
		patch = []map[string]any{
			{"op": "add", "path": "/data", "value": map[string]any{valueToAdd.Name: encodedValue}},
			{"op": "replace", "path": "/metadata/annotations", "value": annotations(actor.User.Identity())},
		}
	} else {
		patch = []map[string]any{
			{"op": "add", "path": "/data/" + escapeJSONPointer(valueToAdd.Name), "value": encodedValue},
			{"op": "replace", "path": "/metadata/annotations", "value": annotations(actor.User.Identity())},
		}
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("marshaling patch: %w", err)
	}

	_, err = client.Namespace(teamSlug.String()).Patch(ctx, secretName, types.JSONPatchType, patchBytes, v1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("patching secret: %w", err)
	}

	err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activityLogEntryActionAddSecretValue,
		Actor:           actor.User,
		EnvironmentName: ptr.To(environment),
		ResourceType:    activityLogEntryResourceTypeSecret,
		ResourceName:    secretName,
		TeamSlug:        ptr.To(teamSlug),
		Data: &SecretValueAddedActivityLogEntryData{
			ValueName: valueToAdd.Name,
		},
	})
	if err != nil {
		fromContext(ctx).log.WithError(err).Errorf("unable to create activity log entry")
	}

	return Get(ctx, teamSlug, environment, secretName)
}

func UpdateSecretValue(ctx context.Context, teamSlug slug.Slug, environment, secretName string, valueToUpdate *SecretValueInput) (*Secret, error) {
	if err := validateSecretValue(valueToUpdate); err != nil {
		return nil, err
	}

	w := fromContext(ctx).Watcher()
	client, err := w.SystemAuthenticatedClient(ctx, environment)
	if err != nil {
		return nil, err
	}

	// Check if the secret exists and is managed by console
	obj, err := client.Namespace(teamSlug.String()).Get(ctx, secretName, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if !secretIsManagedByConsole(obj) {
		return nil, ErrUnmanaged
	}

	// Check if key exists by looking at data keys (not values)
	data, _, _ := unstructured.NestedMap(obj.Object, "data")
	if _, exists := data[valueToUpdate.Name]; !exists {
		return nil, apierror.Errorf("The secret does not contain a secret value with the name %q.", valueToUpdate.Name)
	}

	// Use JSON Patch to update the key without reading other values
	actor := authz.ActorFromContext(ctx)
	encodedValue := base64.StdEncoding.EncodeToString([]byte(valueToUpdate.Value))

	patch := []map[string]any{
		{"op": "replace", "path": "/data/" + escapeJSONPointer(valueToUpdate.Name), "value": encodedValue},
		{"op": "replace", "path": "/metadata/annotations", "value": annotations(actor.User.Identity())},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("marshaling patch: %w", err)
	}

	_, err = client.Namespace(teamSlug.String()).Patch(ctx, secretName, types.JSONPatchType, patchBytes, v1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("patching secret: %w", err)
	}

	err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activityLogEntryActionUpdateSecretValue,
		Actor:           actor.User,
		EnvironmentName: ptr.To(environment),
		ResourceType:    activityLogEntryResourceTypeSecret,
		ResourceName:    secretName,
		TeamSlug:        ptr.To(teamSlug),
		Data: &SecretValueUpdatedActivityLogEntryData{
			ValueName: valueToUpdate.Name,
		},
	})
	if err != nil {
		fromContext(ctx).log.WithError(err).Errorf("unable to create activity log entry")
	}

	return Get(ctx, teamSlug, environment, secretName)
}

func RemoveSecretValue(ctx context.Context, teamSlug slug.Slug, environment, secretName, valueName string) (*Secret, error) {
	w := fromContext(ctx).Watcher()
	client, err := w.SystemAuthenticatedClient(ctx, environment)
	if err != nil {
		return nil, err
	}

	// Check if the secret exists and is managed by console
	obj, err := client.Namespace(teamSlug.String()).Get(ctx, secretName, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if !secretIsManagedByConsole(obj) {
		return nil, ErrUnmanaged
	}

	// Check if key exists by looking at data keys (not values)
	data, _, _ := unstructured.NestedMap(obj.Object, "data")
	if _, exists := data[valueName]; !exists {
		return nil, apierror.Errorf("The secret does not contain a secret value with the name: %q.", valueName)
	}

	// Use JSON Patch to remove the key without reading values
	actor := authz.ActorFromContext(ctx)

	patch := []map[string]any{
		{"op": "remove", "path": "/data/" + escapeJSONPointer(valueName)},
		{"op": "replace", "path": "/metadata/annotations", "value": annotations(actor.User.Identity())},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("marshaling patch: %w", err)
	}

	_, err = client.Namespace(teamSlug.String()).Patch(ctx, secretName, types.JSONPatchType, patchBytes, v1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("patching secret: %w", err)
	}

	err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activityLogEntryActionRemoveSecretValue,
		Actor:           actor.User,
		EnvironmentName: ptr.To(environment),
		ResourceType:    activityLogEntryResourceTypeSecret,
		ResourceName:    secretName,
		TeamSlug:        ptr.To(teamSlug),
		Data: &SecretValueRemovedActivityLogEntryData{
			ValueName: valueName,
		},
	})
	if err != nil {
		fromContext(ctx).log.WithError(err).Errorf("unable to create activity log entry")
	}

	return Get(ctx, teamSlug, environment, secretName)
}

func Delete(ctx context.Context, teamSlug slug.Slug, environment, name string) error {
	w := fromContext(ctx).Watcher()
	client, err := w.SystemAuthenticatedClient(ctx, environment)
	if err != nil {
		return err
	}

	if _, err := client.Namespace(teamSlug.String()).Get(ctx, name, v1.GetOptions{}); err != nil {
		if k8serrors.IsNotFound(err) {
			return &watcher.ErrorNotFound{Cluster: environment, Namespace: teamSlug.String(), Name: name}
		}
		return err
	}

	// Use SystemAuthenticatedClient directly (not watcher.Delete which uses impersonation)
	if err := client.Namespace(teamSlug.String()).Delete(ctx, name, v1.DeleteOptions{}); err != nil {
		return err
	}

	err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activitylog.ActivityLogEntryActionDeleted,
		Actor:           authz.ActorFromContext(ctx).User,
		EnvironmentName: ptr.To(environment),
		ResourceType:    activityLogEntryResourceTypeSecret,
		ResourceName:    name,
		TeamSlug:        ptr.To(teamSlug),
	})
	if err != nil {
		fromContext(ctx).log.WithError(err).Errorf("unable to create activity log entry")
	}

	return nil
}

func annotations(user string) map[string]string {
	m := map[string]string{
		"reloader.stakater.com/match": "true",
	}
	return kubernetes.WithCommonAnnotations(m, user)
}

func validateSecretValue(value *SecretValueInput) error {
	if len(value.Name) > validation.DNS1123SubdomainMaxLength {
		return fmt.Errorf("%q is too long: %d characters, max %d", value.Name, len(value.Name), validation.DNS1123SubdomainMaxLength)
	}

	if isEnvVarName := validation.IsEnvVarName(value.Name); len(isEnvVarName) > 0 {
		return fmt.Errorf("%q: %s", value.Name, strings.Join(isEnvVarName, ", "))
	}

	return nil
}

func secretIsManagedByConsole(secret *unstructured.Unstructured) bool {
	hasConsoleLabel := kubernetes.HasManagedByConsoleLabel(secret)

	secretType, _, _ := unstructured.NestedString(secret.Object, "type")
	isOpaque := secretType == string(corev1.SecretTypeOpaque) || secretType == "kubernetes.io/Opaque"
	hasOwnerReferences := len(secret.GetOwnerReferences()) > 0
	hasFinalizers := len(secret.GetFinalizers()) > 0

	return hasConsoleLabel && isOpaque && !hasOwnerReferences && !hasFinalizers
}

// escapeJSONPointer escapes special characters in JSON Pointer (RFC 6901)
// ~ becomes ~0, / becomes ~1
func escapeJSONPointer(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	s = strings.ReplaceAll(s, "/", "~1")
	return s
}

// ViewSecretValues returns secret values after verifying authorization and logging the access.
// This creates a temporary RBAC elevation and uses impersonation to read the values,
// providing defense in depth (API authorization + Kubernetes RBAC).
func ViewSecretValues(ctx context.Context, input ViewSecretValuesInput) (*ViewSecretValuesPayload, error) {
	// Validate reason
	if len(input.Reason) < 10 {
		return nil, apierror.Errorf("Reason must be at least 10 characters")
	}

	// Check team membership (strict check without admin bypass)
	if err := authz.CanReadSecretValues(ctx, input.Team); err != nil {
		return nil, err
	}

	actor := authz.ActorFromContext(ctx)
	loaders := fromContext(ctx)

	// Create temporary Role and RoleBinding for the user (1 minute TTL)
	elevationID, err := createTemporaryRBAC(ctx, loaders, input, actor)
	if err != nil {
		return nil, fmt.Errorf("creating temporary RBAC: %w", err)
	}

	// Use impersonated client to read secret values (defense in depth)
	impersonatedClient, err := loaders.Client(ctx, input.Environment)
	if err != nil {
		return nil, fmt.Errorf("creating impersonated client: %w", err)
	}

	u, err := impersonatedClient.Namespace(input.Team.String()).Get(ctx, input.Name, v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("reading secret: %w", err)
	}

	data, _, err := unstructured.NestedStringMap(u.Object, "data")
	if err != nil {
		return nil, err
	}

	values := make([]*SecretValue, 0, len(data))
	for k, v := range data {
		val, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, err
		}

		values = append(values, &SecretValue{
			Name:  k,
			Value: string(val),
		})
	}

	slices.SortFunc(values, func(a, b *SecretValue) int {
		return strings.Compare(a.Name, b.Name)
	})

	// Log the access to activity log
	err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activityLogEntryActionViewSecretValues,
		Actor:           actor.User,
		EnvironmentName: ptr.To(input.Environment),
		ResourceType:    activityLogEntryResourceTypeSecret,
		ResourceName:    input.Name,
		TeamSlug:        ptr.To(input.Team),
		Data: &SecretValuesViewedActivityLogEntryData{
			Reason:      input.Reason,
			ElevationID: elevationID,
		},
	})
	if err != nil {
		loaders.log.WithError(err).Errorf("unable to create activity log entry")
	}

	return &ViewSecretValuesPayload{
		Values: values,
	}, nil
}

var (
	roleGVR        = schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"}
	roleBindingGVR = schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"}
)

// createTemporaryRBAC creates a temporary Role and RoleBinding for reading a specific secret.
// The RBAC resources have a 1 minute TTL and will be cleaned up by euthanaisa.
func createTemporaryRBAC(ctx context.Context, loaders *loaders, input ViewSecretValuesInput, actor *authz.Actor) (string, error) {
	k8sClient, exists := loaders.K8sClient(input.Environment)
	if !exists {
		return "", apierror.Errorf("Environment %q does not exist.", input.Environment)
	}

	elevationID := fmt.Sprintf("view-secret-%s", uuid.New().String()[:8])
	namespace := input.Team.String()
	createdAt := time.Now()
	expiresAt := createdAt.Add(1 * time.Minute) // Short TTL - just enough to read the secret

	// Create Role
	role := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "Role",
			"metadata": map[string]any{
				"name":      elevationID,
				"namespace": namespace,
				"labels": map[string]any{
					"nais.io/elevation":             "true",
					"nais.io/elevation-type":        "SECRET",
					"euthanaisa.nais.io/kill-after": strconv.FormatInt(expiresAt.Unix(), 10),
				},
				"annotations": map[string]any{
					"nais.io/elevation-resource": input.Name,
					"nais.io/elevation-user":     actor.User.Identity(),
					"nais.io/elevation-reason":   input.Reason,
					"nais.io/elevation-created":  createdAt.Format(time.RFC3339),
				},
			},
			"rules": []any{
				map[string]any{
					"apiGroups":     []any{""},
					"resources":     []any{"secrets"},
					"verbs":         []any{"get"},
					"resourceNames": []any{input.Name},
				},
			},
		},
	}

	_, err := k8sClient.Resource(roleGVR).Namespace(namespace).Create(ctx, role, v1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("creating role: %w", err)
	}

	// Create RoleBinding
	roleBinding := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "RoleBinding",
			"metadata": map[string]any{
				"name":      elevationID,
				"namespace": namespace,
				"labels": map[string]any{
					"nais.io/elevation":             "true",
					"euthanaisa.nais.io/kill-after": strconv.FormatInt(expiresAt.Unix(), 10),
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

	_, err = k8sClient.Resource(roleBindingGVR).Namespace(namespace).Create(ctx, roleBinding, v1.CreateOptions{})
	if err != nil {
		// Clean up the role if rolebinding creation fails
		_ = k8sClient.Resource(roleGVR).Namespace(namespace).Delete(ctx, elevationID, v1.DeleteOptions{})
		return "", fmt.Errorf("creating rolebinding: %w", err)
	}

	return elevationID, nil
}

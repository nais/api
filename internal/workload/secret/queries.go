package secret

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

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
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"
)

func ListForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName string, workload workload.Workload, page *pagination.Pagination) (*SecretConnection, error) {
	client, err := fromContext(ctx).ServiceAccountClient(environmentName)
	if err != nil {
		return nil, err
	}

	all, err := client.Namespace(teamSlug.String()).List(ctx, v1.ListOptions{
		LabelSelector: kubernetes.IsManagedByConsoleLabelSelector(),
	})
	if err != nil {
		return nil, fmt.Errorf("listing secret: %w", err)
	}

	secretNames := workload.GetSecrets()

	ret := make([]*Secret, 0, len(all.Items))
	for _, u := range all.Items {
		if !slices.Contains(secretNames, u.GetName()) {
			continue
		}
		s, ok := toGraphSecret(&u, environmentName)
		if !ok {
			continue
		}
		ret = append(ret, s)
	}

	SortFilter.Sort(ctx, ret, "NAME", model.OrderDirectionAsc)
	paginated := pagination.Slice(ret, page)
	return pagination.NewConnection(paginated, page, len(ret)), nil
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *SecretOrder, filter *SecretFilter) (*SecretConnection, error) {
	clients, err := fromContext(ctx).ServiceAccountClients()
	if err != nil {
		return nil, err
	}

	retVal := make([]*Secret, 0)
	for env, client := range clients {
		secrets, err := client.Namespace(teamSlug.String()).List(ctx, v1.ListOptions{
			LabelSelector: kubernetes.IsManagedByConsoleLabelSelector(),
		})

		if k8serrors.IsForbidden(err) {
			fromContext(ctx).log.WithFields(logrus.Fields{
				"team":        teamSlug,
				"environment": env,
			}).Infof("skipping secrets listing due to forbidden error")
			continue
		}

		if err != nil {
			return nil, fmt.Errorf("listing secrets for environment %q: %w", env, err)
		}

		for _, u := range secrets.Items {
			s, ok := toGraphSecret(&u, env)
			if !ok {
				continue
			}
			retVal = append(retVal, s)
		}
	}

	if orderBy == nil {
		orderBy = &SecretOrder{
			Field:     "NAME",
			Direction: model.OrderDirectionAsc,
		}
	}

	retVal = SortFilter.Filter(ctx, retVal, filter)
	SortFilter.Sort(ctx, retVal, orderBy.Field, orderBy.Direction)

	secrets := pagination.Slice(retVal, page)
	return pagination.NewConnection(secrets, page, len(retVal)), nil
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*Secret, error) {
	client, err := fromContext(ctx).ServiceAccountClient(environment)
	if err != nil {
		return nil, err
	}

	u, err := client.Namespace(teamSlug.String()).Get(ctx, name, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	s, ok := toGraphSecret(u, environment)
	if !ok {
		return nil, &watcher.ErrorNotFound{Cluster: environment, Namespace: teamSlug.String(), Name: name}
	}
	return s, nil
}

func GetByIdent(ctx context.Context, id ident.Ident) (*Secret, error) {
	teamSlug, env, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, teamSlug, env, name)
}

// GetSecretKeys returns only the key names from a secret using the service account client.
// This does not require elevation as it does not return the actual values.
func GetSecretKeys(ctx context.Context, teamSlug slug.Slug, environmentName, name string) ([]string, error) {
	client, err := fromContext(ctx).ServiceAccountClient(environmentName)
	if err != nil {
		return nil, err
	}

	u, err := client.Namespace(teamSlug.String()).Get(ctx, name, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	data, _, _ := unstructured.NestedMap(u.Object, "data")

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	slices.Sort(keys)

	return keys, nil
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
	client, err := fromContext(ctx).ServiceAccountClient(environment)
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

	client, err := fromContext(ctx).ServiceAccountClient(environment)
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

	client, err := fromContext(ctx).ServiceAccountClient(environment)
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
	client, err := fromContext(ctx).ServiceAccountClient(environment)
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
	client, err := fromContext(ctx).ServiceAccountClient(environment)
	if err != nil {
		return err
	}

	if _, err := client.Namespace(teamSlug.String()).Get(ctx, name, v1.GetOptions{}); err != nil {
		if k8serrors.IsNotFound(err) {
			return &watcher.ErrorNotFound{Cluster: environment, Namespace: teamSlug.String(), Name: name}
		}
		return err
	}

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

func secretTupleToMap(data []*SecretValue) map[string][]byte {
	ret := make(map[string][]byte, len(data))
	for _, tuple := range data {
		ret[tuple.Name] = []byte(tuple.Value)
	}
	return ret
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

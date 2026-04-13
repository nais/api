package config

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
	"github.com/nais/api/internal/workload/secret"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
)

func ListForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName string, w workload.Workload, page *pagination.Pagination) (*ConfigConnection, error) {
	configNames := w.GetConfigs()
	allConfigs := watcher.Objects(fromContext(ctx).Watcher().GetByNamespace(teamSlug.String(), watcher.InCluster(environmentName)))

	ret := make([]*Config, 0, len(allConfigs))
	for _, c := range allConfigs {
		if slices.Contains(configNames, c.Name) {
			ret = append(ret, c)
		}
	}

	SortFilter.Sort(ctx, ret, "NAME", model.OrderDirectionAsc)
	paginated := pagination.Slice(ret, page)
	return pagination.NewConnection(paginated, page, len(ret)), nil
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *ConfigOrder, filter *ConfigFilter) (*ConfigConnection, error) {
	allConfigs := watcher.Objects(fromContext(ctx).Watcher().GetByNamespace(teamSlug.String()))

	if orderBy == nil {
		orderBy = &ConfigOrder{
			Field:     "NAME",
			Direction: model.OrderDirectionAsc,
		}
	}

	filtered := SortFilter.Filter(ctx, allConfigs, filter)
	SortFilter.Sort(ctx, filtered, orderBy.Field, orderBy.Direction)

	configs := pagination.Slice(filtered, page)
	return pagination.NewConnection(configs, page, len(filtered)), nil
}

func CountForTeam(ctx context.Context, teamSlug slug.Slug) int {
	return len(fromContext(ctx).Watcher().GetByNamespace(teamSlug.String()))
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*Config, error) {
	config, err := fromContext(ctx).Watcher().Get(environment, teamSlug.String(), name)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func GetByIdent(ctx context.Context, id ident.Ident) (*Config, error) {
	teamSlug, env, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, teamSlug, env, name)
}

// GetConfigValues returns the config values directly from the cached data.
// ConfigMap data is not sensitive, so no elevation is needed.
func GetConfigValues(ctx context.Context, teamSlug slug.Slug, environmentName, name string) ([]*ConfigValue, error) {
	config, err := fromContext(ctx).Watcher().Get(environmentName, teamSlug.String(), name)
	if err != nil {
		return nil, err
	}

	values := configValuesFromConfig(config)

	slices.SortFunc(values, func(a, b *ConfigValue) int {
		return strings.Compare(a.Name, b.Name)
	})

	return values, nil
}

// configValuesFromConfig builds ConfigValue entries from a Config's Data and BinaryData fields.
// Values in Data are plain text; values in BinaryData are base64-encoded binary.
func configValuesFromConfig(config *Config) []*ConfigValue {
	values := make([]*ConfigValue, 0, len(config.Data)+len(config.BinaryData))
	for k, v := range config.Data {
		values = append(values, &ConfigValue{
			Name:     k,
			Value:    v,
			Encoding: secret.ValueEncodingPlainText,
		})
	}
	for k, v := range config.BinaryData {
		values = append(values, &ConfigValue{
			Name:     k,
			Value:    v,
			Encoding: secret.ValueEncodingBase64,
		})
	}
	return values
}

func Create(ctx context.Context, teamSlug slug.Slug, environment, name string) (*Config, error) {
	w := fromContext(ctx).Watcher()
	client, err := w.ImpersonatedClient(ctx, environment)
	if err != nil {
		return nil, err
	}

	if nameErrs := validation.IsDNS1123Subdomain(name); len(nameErrs) > 0 {
		return nil, fmt.Errorf("invalid name %q: %s", name, strings.Join(nameErrs, ", "))
	}

	actor := authz.ActorFromContext(ctx)

	cm := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:        name,
			Namespace:   teamSlug.String(),
			Annotations: annotations(actor.User.Identity()),
		},
	}

	kubernetes.SetManagedByConsoleLabel(cm)

	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
	if err != nil {
		return nil, err
	}

	created, err := client.Namespace(teamSlug.String()).Create(ctx, &unstructured.Unstructured{Object: u}, v1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return nil, ErrAlreadyExists
		}
		return nil, fmt.Errorf("creating config: %w", err)
	}

	err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activitylog.ActivityLogEntryActionCreated,
		Actor:           actor.User,
		EnvironmentName: &environment,
		ResourceType:    activityLogEntryResourceTypeConfig,
		ResourceName:    name,
		TeamSlug:        &teamSlug,
	})
	if err != nil {
		fromContext(ctx).log.WithError(err).Errorf("unable to create activity log entry")
	}

	retVal, ok := toGraphConfig(created, environment)
	if !ok {
		return nil, fmt.Errorf("failed to convert config")
	}
	return retVal, nil
}

func AddConfigValue(ctx context.Context, teamSlug slug.Slug, environment, configName string, valueToAdd *ConfigValueInput) (*Config, error) {
	if err := validateConfigValue(valueToAdd); err != nil {
		return nil, err
	}

	w := fromContext(ctx).Watcher()
	client, err := w.ImpersonatedClient(ctx, environment)
	if err != nil {
		return nil, err
	}

	// Check if the config exists and is managed by console
	obj, err := client.Namespace(teamSlug.String()).Get(ctx, configName, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if !configIsManagedByConsole(obj) {
		return nil, ErrUnmanaged
	}

	// Check if key already exists in either data or binaryData
	data, dataExists, _ := unstructured.NestedMap(obj.Object, "data")
	binaryData, binaryDataExists, _ := unstructured.NestedMap(obj.Object, "binaryData")
	if _, exists := data[valueToAdd.Name]; exists {
		return nil, apierror.Errorf("The config already contains a value with the name %q.", valueToAdd.Name)
	}
	if _, exists := binaryData[valueToAdd.Name]; exists {
		return nil, apierror.Errorf("The config already contains a value with the name %q.", valueToAdd.Name)
	}

	// Encode the value for storage
	encodedValue, err := encodeValueForStorage(valueToAdd)
	if err != nil {
		return nil, err
	}

	// Use JSON Patch to add the new key
	actor := authz.ActorFromContext(ctx)

	isBinary := valueToAdd.Encoding != nil && *valueToAdd.Encoding == secret.ValueEncodingBase64
	mergedAnnotations := mergeAnnotations(obj.GetAnnotations(), actor.User.Identity(), nil)

	var patch []map[string]any
	if isBinary {
		if !binaryDataExists || binaryData == nil {
			patch = []map[string]any{
				{"op": "add", "path": "/binaryData", "value": map[string]any{valueToAdd.Name: encodedValue}},
				{"op": "replace", "path": "/metadata/annotations", "value": mergedAnnotations},
			}
		} else {
			patch = []map[string]any{
				{"op": "add", "path": "/binaryData/" + escapeJSONPointer(valueToAdd.Name), "value": encodedValue},
				{"op": "replace", "path": "/metadata/annotations", "value": mergedAnnotations},
			}
		}
	} else {
		if !dataExists || data == nil {
			patch = []map[string]any{
				{"op": "add", "path": "/data", "value": map[string]any{valueToAdd.Name: encodedValue}},
				{"op": "replace", "path": "/metadata/annotations", "value": mergedAnnotations},
			}
		} else {
			patch = []map[string]any{
				{"op": "add", "path": "/data/" + escapeJSONPointer(valueToAdd.Name), "value": encodedValue},
				{"op": "replace", "path": "/metadata/annotations", "value": mergedAnnotations},
			}
		}
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("marshaling patch: %w", err)
	}

	_, err = client.Namespace(teamSlug.String()).Patch(ctx, configName, types.JSONPatchType, patchBytes, v1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("patching config: %w", err)
	}

	// Don't log actual values for binary keys — they are meaningless base64 strings
	addedField := &ConfigUpdatedActivityLogEntryDataUpdatedField{
		Field: valueToAdd.Name,
	}
	if !isBinary {
		addedField.NewValue = &valueToAdd.Value
	}

	err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activitylog.ActivityLogEntryActionUpdated,
		Actor:           actor.User,
		EnvironmentName: &environment,
		ResourceType:    activityLogEntryResourceTypeConfig,
		ResourceName:    configName,
		TeamSlug:        &teamSlug,
		Data: ConfigUpdatedActivityLogEntryData{
			UpdatedFields: []*ConfigUpdatedActivityLogEntryDataUpdatedField{addedField},
		},
	})
	if err != nil {
		fromContext(ctx).log.WithError(err).Errorf("unable to create activity log entry")
	}

	// Re-fetch from the K8s API to return up-to-date data
	updated, err := client.Namespace(teamSlug.String()).Get(ctx, configName, v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("fetching updated config: %w", err)
	}

	retVal, ok := toGraphConfig(updated, environment)
	if !ok {
		return nil, fmt.Errorf("failed to convert config")
	}
	return retVal, nil
}

func UpdateConfigValue(ctx context.Context, teamSlug slug.Slug, environment, configName string, valueToUpdate *ConfigValueInput) (*Config, error) {
	if err := validateConfigValue(valueToUpdate); err != nil {
		return nil, err
	}

	w := fromContext(ctx).Watcher()
	client, err := w.ImpersonatedClient(ctx, environment)
	if err != nil {
		return nil, err
	}

	// Check if the config exists and is managed by console
	obj, err := client.Namespace(teamSlug.String()).Get(ctx, configName, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if !configIsManagedByConsole(obj) {
		return nil, ErrUnmanaged
	}

	// Check if key exists in data or binaryData
	data, dataExists, _ := unstructured.NestedMap(obj.Object, "data")
	binaryData, binaryDataExists, _ := unstructured.NestedMap(obj.Object, "binaryData")

	oldValueRaw, existsInData := data[valueToUpdate.Name]
	_, existsInBinaryData := binaryData[valueToUpdate.Name]
	if !existsInData && !existsInBinaryData {
		return nil, apierror.Errorf("The config does not contain a value with the name %q.", valueToUpdate.Name)
	}

	wasBinary := existsInBinaryData

	var oldValue *string
	if !wasBinary {
		if s, ok := oldValueRaw.(string); ok {
			oldValue = &s
		}
	}

	// Encode the value for storage
	encodedValue, err := encodeValueForStorage(valueToUpdate)
	if err != nil {
		return nil, err
	}

	// Use JSON Patch to update the key, moving between data/binaryData if encoding changed
	actor := authz.ActorFromContext(ctx)

	isBinary := valueToUpdate.Encoding != nil && *valueToUpdate.Encoding == secret.ValueEncodingBase64
	mergedAnnotations := mergeAnnotations(obj.GetAnnotations(), actor.User.Identity(), nil)

	escapedName := escapeJSONPointer(valueToUpdate.Name)
	var patch []map[string]any
	switch {
	case wasBinary && !isBinary:
		// Move from binaryData to data
		patch = []map[string]any{
			{"op": "remove", "path": "/binaryData/" + escapedName},
		}
		if !dataExists || data == nil {
			patch = append(patch, map[string]any{"op": "add", "path": "/data", "value": map[string]any{valueToUpdate.Name: encodedValue}})
		} else {
			patch = append(patch, map[string]any{"op": "add", "path": "/data/" + escapedName, "value": encodedValue})
		}
	case !wasBinary && isBinary:
		// Move from data to binaryData
		patch = []map[string]any{
			{"op": "remove", "path": "/data/" + escapedName},
		}
		if !binaryDataExists || binaryData == nil {
			patch = append(patch, map[string]any{"op": "add", "path": "/binaryData", "value": map[string]any{valueToUpdate.Name: encodedValue}})
		} else {
			patch = append(patch, map[string]any{"op": "add", "path": "/binaryData/" + escapedName, "value": encodedValue})
		}
	default:
		// Encoding unchanged — replace in the same field
		field := "data"
		if isBinary {
			field = "binaryData"
		}
		patch = []map[string]any{
			{"op": "replace", "path": "/" + field + "/" + escapedName, "value": encodedValue},
		}
	}
	patch = append(patch, map[string]any{"op": "replace", "path": "/metadata/annotations", "value": mergedAnnotations})

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("marshaling patch: %w", err)
	}

	_, err = client.Namespace(teamSlug.String()).Patch(ctx, configName, types.JSONPatchType, patchBytes, v1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("patching config: %w", err)
	}

	// Don't log actual values for binary keys — they are meaningless base64 strings
	updatedField := &ConfigUpdatedActivityLogEntryDataUpdatedField{
		Field: valueToUpdate.Name,
	}
	if !isBinary && !wasBinary {
		updatedField.OldValue = oldValue
		updatedField.NewValue = &valueToUpdate.Value
	}

	err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activitylog.ActivityLogEntryActionUpdated,
		Actor:           actor.User,
		EnvironmentName: &environment,
		ResourceType:    activityLogEntryResourceTypeConfig,
		ResourceName:    configName,
		TeamSlug:        &teamSlug,
		Data: ConfigUpdatedActivityLogEntryData{
			UpdatedFields: []*ConfigUpdatedActivityLogEntryDataUpdatedField{updatedField},
		},
	})
	if err != nil {
		fromContext(ctx).log.WithError(err).Errorf("unable to create activity log entry")
	}

	// Re-fetch from the K8s API to return up-to-date data
	updated, err := client.Namespace(teamSlug.String()).Get(ctx, configName, v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("fetching updated config: %w", err)
	}

	retVal, ok := toGraphConfig(updated, environment)
	if !ok {
		return nil, fmt.Errorf("failed to convert config")
	}
	return retVal, nil
}

func RemoveConfigValue(ctx context.Context, teamSlug slug.Slug, environment, configName, valueName string) (*Config, error) {
	w := fromContext(ctx).Watcher()
	client, err := w.ImpersonatedClient(ctx, environment)
	if err != nil {
		return nil, err
	}

	// Check if the config exists and is managed by console
	obj, err := client.Namespace(teamSlug.String()).Get(ctx, configName, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if !configIsManagedByConsole(obj) {
		return nil, ErrUnmanaged
	}

	// Check if key exists in data or binaryData
	data, _, _ := unstructured.NestedMap(obj.Object, "data")
	binaryData, _, _ := unstructured.NestedMap(obj.Object, "binaryData")

	oldValueRaw, existsInData := data[valueName]
	_, existsInBinaryData := binaryData[valueName]
	if !existsInData && !existsInBinaryData {
		return nil, apierror.Errorf("The config does not contain a value with the name: %q.", valueName)
	}

	isBinary := existsInBinaryData

	var oldValue *string
	if !isBinary {
		if s, ok := oldValueRaw.(string); ok {
			oldValue = &s
		}
	}

	// Use JSON Patch to remove the key from the appropriate field
	actor := authz.ActorFromContext(ctx)

	mergedAnnotations := mergeAnnotations(obj.GetAnnotations(), actor.User.Identity(), nil)

	field := "data"
	if isBinary {
		field = "binaryData"
	}
	patch := []map[string]any{
		{"op": "remove", "path": "/" + field + "/" + escapeJSONPointer(valueName)},
		{"op": "replace", "path": "/metadata/annotations", "value": mergedAnnotations},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("marshaling patch: %w", err)
	}

	_, err = client.Namespace(teamSlug.String()).Patch(ctx, configName, types.JSONPatchType, patchBytes, v1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("patching config: %w", err)
	}

	err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activitylog.ActivityLogEntryActionUpdated,
		Actor:           actor.User,
		EnvironmentName: &environment,
		ResourceType:    activityLogEntryResourceTypeConfig,
		ResourceName:    configName,
		TeamSlug:        &teamSlug,
		Data: ConfigUpdatedActivityLogEntryData{
			UpdatedFields: []*ConfigUpdatedActivityLogEntryDataUpdatedField{
				{
					Field:    valueName,
					OldValue: oldValue,
				},
			},
		},
	})
	if err != nil {
		fromContext(ctx).log.WithError(err).Errorf("unable to create activity log entry")
	}

	// Re-fetch from the K8s API to return up-to-date data
	updated, err := client.Namespace(teamSlug.String()).Get(ctx, configName, v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("fetching updated config: %w", err)
	}

	retVal, ok := toGraphConfig(updated, environment)
	if !ok {
		return nil, fmt.Errorf("failed to convert config")
	}
	return retVal, nil
}

func Delete(ctx context.Context, teamSlug slug.Slug, environment, name string) error {
	w := fromContext(ctx).Watcher()
	client, err := w.ImpersonatedClient(ctx, environment)
	if err != nil {
		return err
	}

	obj, err := client.Namespace(teamSlug.String()).Get(ctx, name, v1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return &watcher.ErrorNotFound{Cluster: environment, Namespace: teamSlug.String(), Name: name}
		}
		return err
	}

	if !configIsManagedByConsole(obj) {
		return ErrUnmanaged
	}

	if err := client.Namespace(teamSlug.String()).Delete(ctx, name, v1.DeleteOptions{}); err != nil {
		return err
	}

	err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activitylog.ActivityLogEntryActionDeleted,
		Actor:           authz.ActorFromContext(ctx).User,
		EnvironmentName: &environment,
		ResourceType:    activityLogEntryResourceTypeConfig,
		ResourceName:    name,
		TeamSlug:        &teamSlug,
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

func validateConfigValue(value *ConfigValueInput) error {
	if errs := validation.IsConfigMapKey(value.Name); len(errs) > 0 {
		return fmt.Errorf("invalid config key %q: %s", value.Name, strings.Join(errs, ", "))
	}

	return nil
}

func configIsManagedByConsole(cm *unstructured.Unstructured) bool {
	hasConsoleLabel := kubernetes.HasManagedByConsoleLabel(cm)
	hasOwnerReferences := len(cm.GetOwnerReferences()) > 0
	hasFinalizers := len(cm.GetFinalizers()) > 0

	return hasConsoleLabel && !hasOwnerReferences && !hasFinalizers
}

// escapeJSONPointer escapes special characters in JSON Pointer (RFC 6901)
// ~ becomes ~0, / becomes ~1
func escapeJSONPointer(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	s = strings.ReplaceAll(s, "/", "~1")
	return s
}

// encodeValueForStorage prepares a config value for storage in a Kubernetes ConfigMap.
// When encoding is BASE64, the client sends base64-encoded binary data which will be
// stored in the binaryData field. We validate the base64 and return as-is.
// When encoding is PLAIN_TEXT (or unset), the value is stored in the data field as-is.
func encodeValueForStorage(input *ConfigValueInput) (string, error) {
	encoding := secret.ValueEncodingPlainText
	if input.Encoding != nil {
		encoding = *input.Encoding
	}

	switch encoding {
	case secret.ValueEncodingBase64:
		// Validate that the value is valid base64
		if _, err := base64.StdEncoding.DecodeString(input.Value); err != nil {
			return "", fmt.Errorf("value is not valid base64: %w", err)
		}
		// The value is already base64-encoded, which is what Kubernetes binaryData expects
		return input.Value, nil
	case secret.ValueEncodingPlainText:
		return input.Value, nil
	default:
		return "", fmt.Errorf("unsupported encoding: %s", encoding)
	}
}

// mergeAnnotations returns annotations for a JSON Patch that preserves existing annotations
// while updating the standard ones (last-modified-at, etc.).
func mergeAnnotations(existingAnnotations map[string]string, user string, extraAnnotations map[string]string) map[string]string {
	merged := make(map[string]string)
	// Start with existing annotations
	for k, v := range existingAnnotations {
		merged[k] = v
	}
	// Apply standard annotations (overwrites last-modified-at, etc.)
	for k, v := range annotations(user) {
		merged[k] = v
	}
	// Apply extra annotations
	for k, v := range extraAnnotations {
		if v == "" {
			delete(merged, k)
		} else {
			merged[k] = v
		}
	}
	return merged
}

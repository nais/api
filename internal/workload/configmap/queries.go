package configmap

import (
	"context"
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

	values := make([]*ConfigValue, 0, len(config.Data))
	for k, v := range config.Data {
		values = append(values, &ConfigValue{
			Name:  k,
			Value: v,
		})
	}

	slices.SortFunc(values, func(a, b *ConfigValue) int {
		return strings.Compare(a.Name, b.Name)
	})

	return values, nil
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
		return nil, fmt.Errorf("failed to convert configmap to graph config")
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

	// Check if the configmap exists and is managed by console
	obj, err := client.Namespace(teamSlug.String()).Get(ctx, configName, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if !configIsManagedByConsole(obj) {
		return nil, ErrUnmanaged
	}

	// Check if key already exists
	data, dataExists, _ := unstructured.NestedMap(obj.Object, "data")
	if _, exists := data[valueToAdd.Name]; exists {
		return nil, apierror.Errorf("The config already contains a value with the name %q.", valueToAdd.Name)
	}

	// Use JSON Patch to add the new key
	actor := authz.ActorFromContext(ctx)

	var patch []map[string]any
	if !dataExists || data == nil {
		patch = []map[string]any{
			{"op": "add", "path": "/data", "value": map[string]any{valueToAdd.Name: valueToAdd.Value}},
			{"op": "replace", "path": "/metadata/annotations", "value": annotations(actor.User.Identity())},
		}
	} else {
		patch = []map[string]any{
			{"op": "add", "path": "/data/" + escapeJSONPointer(valueToAdd.Name), "value": valueToAdd.Value},
			{"op": "replace", "path": "/metadata/annotations", "value": annotations(actor.User.Identity())},
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
					Field:    valueToAdd.Name,
					NewValue: &valueToAdd.Value,
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
		return nil, fmt.Errorf("failed to convert configmap")
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

	// Check if the configmap exists and is managed by console
	obj, err := client.Namespace(teamSlug.String()).Get(ctx, configName, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if !configIsManagedByConsole(obj) {
		return nil, ErrUnmanaged
	}

	// Check if key exists
	data, _, _ := unstructured.NestedMap(obj.Object, "data")
	oldValueRaw, exists := data[valueToUpdate.Name]
	if !exists {
		return nil, apierror.Errorf("The config does not contain a value with the name %q.", valueToUpdate.Name)
	}

	var oldValue *string
	if s, ok := oldValueRaw.(string); ok {
		oldValue = &s
	}

	// Use JSON Patch to update the key
	actor := authz.ActorFromContext(ctx)

	patch := []map[string]any{
		{"op": "replace", "path": "/data/" + escapeJSONPointer(valueToUpdate.Name), "value": valueToUpdate.Value},
		{"op": "replace", "path": "/metadata/annotations", "value": annotations(actor.User.Identity())},
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
					Field:    valueToUpdate.Name,
					OldValue: oldValue,
					NewValue: &valueToUpdate.Value,
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
		return nil, fmt.Errorf("failed to convert configmap")
	}
	return retVal, nil
}

func RemoveConfigValue(ctx context.Context, teamSlug slug.Slug, environment, configName, valueName string) (*Config, error) {
	w := fromContext(ctx).Watcher()
	client, err := w.ImpersonatedClient(ctx, environment)
	if err != nil {
		return nil, err
	}

	// Check if the configmap exists and is managed by console
	obj, err := client.Namespace(teamSlug.String()).Get(ctx, configName, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if !configIsManagedByConsole(obj) {
		return nil, ErrUnmanaged
	}

	// Check if key exists
	data, _, _ := unstructured.NestedMap(obj.Object, "data")
	oldValueRaw, exists := data[valueName]
	if !exists {
		return nil, apierror.Errorf("The config does not contain a value with the name: %q.", valueName)
	}

	var oldValue *string
	if s, ok := oldValueRaw.(string); ok {
		oldValue = &s
	}

	// Use JSON Patch to remove the key
	actor := authz.ActorFromContext(ctx)

	patch := []map[string]any{
		{"op": "remove", "path": "/data/" + escapeJSONPointer(valueName)},
		{"op": "replace", "path": "/metadata/annotations", "value": annotations(actor.User.Identity())},
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
		return nil, fmt.Errorf("failed to convert configmap")
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

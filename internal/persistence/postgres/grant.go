package postgres

import (
	"context"
	"fmt"
	"hash/crc32"
	"strconv"
	"time"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// Keep the legacy Zalando port-forward grant independent of the PostgresBranch CRD.
// It uses API's existing system clients; only this mutation reads data.nais.io.
func legacyPostgresClient(ctx context.Context, environment string, gvr schema.GroupVersionResource) (dynamic.NamespaceableResourceInterface, error) {
	client, ok := fromContext(ctx).clients[environmentmapper.ClusterName(environment)]
	if !ok {
		return nil, &watcher.ErrorUnknownEnvironment{Environment: environment}
	}
	return client.Resource(gvr), nil
}

func getLegacyPostgres(ctx context.Context, input GrantPostgresAccessInput) error {
	gvr := schema.GroupVersionResource{Group: "data.nais.io", Version: "v1", Resource: "postgres"}
	client, err := legacyPostgresClient(ctx, input.EnvironmentName, gvr)
	if err != nil {
		return err
	}
	_, err = client.Namespace(input.TeamSlug.String()).Get(ctx, input.ClusterName, metav1.GetOptions{})
	return err
}

func GrantZalandoPostgresAccess(ctx context.Context, input GrantPostgresAccessInput) error {
	if err := input.Validate(ctx); err != nil {
		return err
	}

	namespace := fmt.Sprintf("pg-%s", input.TeamSlug.String())
	name := resourceNamer(input.TeamSlug, input.Grantee, input.ClusterName)
	d, err := time.ParseDuration(input.Duration)
	if err != nil {
		return fmt.Errorf("parsing TTL: %w", err)
	}
	until := time.Now().Add(d)
	labels := map[string]string{
		"euthanaisa.nais.io/kill-after": strconv.FormatInt(until.Unix(), 10),
		"postgres.data.nais.io/name":    input.ClusterName,
	}

	if err := createGrantRole(ctx, input, name, namespace, labels); err != nil {
		return err
	}
	if err := createGrantRoleBinding(ctx, input, name, namespace, labels); err != nil {
		return err
	}

	return activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activityLogEntryActionGrantAccess,
		Actor:           authz.ActorFromContext(ctx).User,
		ResourceType:    activityLogEntryResourceTypePostgres,
		ResourceName:    input.ClusterName,
		EnvironmentName: new(input.EnvironmentName),
		TeamSlug:        new(input.TeamSlug),
		Data: PostgresGrantAccessActivityLogEntryData{
			Grantee: input.Grantee,
			Until:   until,
		},
	})
}

func createGrantRoleBinding(ctx context.Context, input GrantPostgresAccessInput, name, namespace string, labels map[string]string) error {
	gvr := schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"}
	client, err := legacyPostgresClient(ctx, input.EnvironmentName, gvr)
	if err != nil {
		return err
	}
	res := &unstructured.Unstructured{}
	res.SetAPIVersion(gvr.GroupVersion().String())
	res.SetKind("RoleBinding")
	res.SetName(name)
	res.SetNamespace(namespace)
	res.SetAnnotations(kubernetes.WithCommonAnnotations(nil, authz.ActorFromContext(ctx).User.Identity()))
	res.SetLabels(labels)
	kubernetes.SetManagedByConsoleLabel(res)
	res.Object["roleRef"] = map[string]any{
		"apiGroup": "rbac.authorization.k8s.io",
		"kind":     "Role",
		"name":     name,
	}
	res.Object["subjects"] = []any{map[string]any{"kind": "User", "name": input.Grantee}}
	return createOrUpdateGrantResource(ctx, res, client.Namespace(namespace))
}

func createGrantRole(ctx context.Context, input GrantPostgresAccessInput, name, namespace string, labels map[string]string) error {
	gvr := schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"}
	client, err := legacyPostgresClient(ctx, input.EnvironmentName, gvr)
	if err != nil {
		return err
	}
	res := &unstructured.Unstructured{}
	res.SetAPIVersion(gvr.GroupVersion().String())
	res.SetKind("Role")
	res.SetName(name)
	res.SetNamespace(namespace)
	res.SetAnnotations(kubernetes.WithCommonAnnotations(nil, authz.ActorFromContext(ctx).User.Identity()))
	res.SetLabels(labels)
	kubernetes.SetManagedByConsoleLabel(res)
	pods := []any{fmt.Sprintf("%s-0", input.ClusterName), fmt.Sprintf("%s-1", input.ClusterName), fmt.Sprintf("%s-2", input.ClusterName)}
	res.Object["rules"] = []any{
		map[string]any{"apiGroups": []any{""}, "resources": []any{"pods"}, "verbs": []any{"get", "list", "watch"}, "resourceNames": pods},
		map[string]any{"apiGroups": []any{""}, "resources": []any{"pods/portforward"}, "verbs": []any{"get", "list", "watch", "create"}, "resourceNames": pods},
	}
	return createOrUpdateGrantResource(ctx, res, client.Namespace(namespace))
}

func createOrUpdateGrantResource(ctx context.Context, res *unstructured.Unstructured, client dynamic.ResourceInterface) error {
	_, err := client.Create(ctx, res, metav1.CreateOptions{})
	if k8serrors.IsAlreadyExists(err) {
		_, err = client.Update(ctx, res, metav1.UpdateOptions{})
	}
	return err
}

func resourceNamer(teamSlug slug.Slug, grantee, name string) string {
	hasher := crc32.NewIEEE()
	fmt.Fprintf(hasher, "%s-%s-%s", teamSlug.String(), grantee, name)
	return fmt.Sprintf("pg-grant-%08x", hasher.Sum32())
}

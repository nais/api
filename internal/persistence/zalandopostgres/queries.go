package zalandopostgres

import (
	"context"
	"fmt"
	"hash/crc32"
	"strconv"
	"time"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/utils/ptr"
)

func GetZalandoPostgresByIdent(ctx context.Context, id ident.Ident) (*ZalandoPostgres, error) {
	teamSlug, environmentName, clusterName, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return GetZalandoPostgres(ctx, teamSlug, environmentName, clusterName)
}

func GetZalandoPostgres(ctx context.Context, teamSlug slug.Slug, environmentName string, clusterName string) (*ZalandoPostgres, error) {
	return fromContext(ctx).zalandoPostgresWatcher.Get(environmentName, teamSlug.String(), clusterName)
}

func GrantZalandoPostgresAccess(ctx context.Context, input GrantZalandoPostgresAccessInput) error {
	err := input.Validate(ctx)
	if err != nil {
		return err
	}

	namespace := fmt.Sprintf("pg-%s", input.TeamSlug.String())
	name, err := resourceNamer(input.TeamSlug, input.Grantee, input.ClusterName)
	if err != nil {
		return err
	}

	annotations := make(map[string]string)
	d, err := time.ParseDuration(input.Duration)
	if err != nil {
		return fmt.Errorf("parsing TTL: %w", err)
	}
	until := time.Now().Add(d)

	labels := make(map[string]string)
	labels["euthanaisa.nais.io/kill-after"] = strconv.FormatInt(until.Unix(), 10)
	labels["postgres.data.nais.io/name"] = input.ClusterName

	err = createRole(ctx, input, name, namespace, annotations, labels)
	if err != nil {
		return err
	}

	err = createRoleBinding(ctx, input, name, namespace, annotations, labels)
	if err != nil {
		return err
	}

	return activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activityLogEntryActionGrantAccess,
		Actor:           authz.ActorFromContext(ctx).User,
		ResourceType:    activityLogEntryResourceTypeZalandoPostgres,
		ResourceName:    input.ClusterName,
		EnvironmentName: ptr.To(input.EnvironmentName),
		TeamSlug:        ptr.To(input.TeamSlug),
		Data: ZalandoPostgresGrantAccessActivityLogEntryData{
			Grantee: input.Grantee,
			Until:   until,
		},
	})
}

func createRoleBinding(ctx context.Context, input GrantZalandoPostgresAccessInput, name string, namespace string, annotations map[string]string, labels map[string]string) error {
	gvr := schema.GroupVersionResource{
		Group:    "rbac.authorization.k8s.io",
		Version:  "v1",
		Resource: "rolebindings",
	}
	client, err := fromContext(ctx).zalandoPostgresWatcher.ImpersonatedClientWithNamespace(ctx, input.EnvironmentName, namespace, watcher.WithImpersonatedClientGVR(gvr))
	if err != nil {
		return err
	}

	res := &unstructured.Unstructured{}
	res.SetAPIVersion(gvr.GroupVersion().String())
	res.SetKind("RoleBinding")
	res.SetName(name)
	res.SetNamespace(namespace)
	res.SetAnnotations(kubernetes.WithCommonAnnotations(annotations, authz.ActorFromContext(ctx).User.Identity()))
	res.SetLabels(labels)
	kubernetes.SetManagedByConsoleLabel(res)

	res.Object["roleRef"] = map[string]any{
		"apiGroup": "rbac.authorization.k8s.io",
		"kind":     "Role",
		"name":     name,
	}

	res.Object["subjects"] = []any{
		map[string]any{
			"kind": "User",
			"name": input.Grantee,
		},
	}

	return createOrUpdateResource(ctx, res, client)
}

func createRole(ctx context.Context, input GrantZalandoPostgresAccessInput, name string, namespace string, annotations map[string]string, labels map[string]string) error {
	gvr := schema.GroupVersionResource{
		Group:    "rbac.authorization.k8s.io",
		Version:  "v1",
		Resource: "roles",
	}
	client, err := fromContext(ctx).zalandoPostgresWatcher.ImpersonatedClientWithNamespace(ctx, input.EnvironmentName, namespace, watcher.WithImpersonatedClientGVR(gvr))
	if err != nil {
		return err
	}

	res := &unstructured.Unstructured{}
	res.SetAPIVersion(gvr.GroupVersion().String())
	res.SetKind("Role")
	res.SetName(name)
	res.SetNamespace(namespace)
	res.SetAnnotations(kubernetes.WithCommonAnnotations(annotations, authz.ActorFromContext(ctx).User.Identity()))
	res.SetLabels(labels)
	kubernetes.SetManagedByConsoleLabel(res)

	res.Object["rules"] = []any{
		map[string]any{
			"apiGroups": []any{""},
			"resources": []any{"pods"},
			"verbs":     []any{"get", "list", "watch"},
			"resourceNames": []any{
				fmt.Sprintf("%s-0", input.ClusterName),
				fmt.Sprintf("%s-1", input.ClusterName),
				fmt.Sprintf("%s-2", input.ClusterName),
			},
		},
		map[string]any{
			"apiGroups": []any{""},
			"resources": []any{"pods/portforward"},
			"verbs":     []any{"get", "list", "watch", "create"},
			"resourceNames": []any{
				fmt.Sprintf("%s-0", input.ClusterName),
				fmt.Sprintf("%s-1", input.ClusterName),
				fmt.Sprintf("%s-2", input.ClusterName),
			},
		},
	}

	return createOrUpdateResource(ctx, res, client)
}

func createOrUpdateResource(ctx context.Context, res *unstructured.Unstructured, client dynamic.ResourceInterface) error {
	_, err := client.Create(ctx, res, metav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			_, err = client.Update(ctx, res, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
			return nil
		}
		return err
	}
	return nil
}

func resourceNamer(teamSlug slug.Slug, grantee string, name string) (string, error) {
	hasher := crc32.NewIEEE()
	_, err := fmt.Fprintf(hasher, "%s-%s-%s", teamSlug.String(), grantee, name)
	if err != nil {
		return "", err
	}
	hashStr := fmt.Sprintf("%08x", hasher.Sum32())
	return fmt.Sprintf("pg-grant-%s", hashStr), nil
}

package sqlinstance

import (
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"time"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	"google.golang.org/api/googleapi"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/utils/ptr"
)

func GetByIdent(ctx context.Context, id ident.Ident) (*SQLInstance, error) {
	teamSlug, environmentName, sqlInstanceName, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return Get(ctx, teamSlug, environmentName, sqlInstanceName)
}

func Get(ctx context.Context, teamSlug slug.Slug, environmentName, sqlInstanceName string) (*SQLInstance, error) {
	return fromContext(ctx).sqlInstanceWatcher.Get(environmentName, teamSlug.String(), sqlInstanceName)
}

func GetDatabaseByIdent(ctx context.Context, id ident.Ident) (*SQLDatabase, error) {
	teamSlug, environmentName, sqlInstanceName, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return GetDatabase(ctx, teamSlug, environmentName, sqlInstanceName)
}

func GetDatabase(ctx context.Context, teamSlug slug.Slug, environmentName, sqlInstanceName string) (*SQLDatabase, error) {
	all := fromContext(ctx).sqlDatabaseWatcher.GetByNamespace(teamSlug.String(), watcher.InCluster(environmentName))

	for _, db := range all {
		if db.Obj.SQLInstanceName == sqlInstanceName {
			return db.Obj, nil
		}
	}

	return nil, &watcher.ErrorNotFound{
		Cluster:   environmentName,
		Namespace: teamSlug.String(),
		Name:      sqlInstanceName,
	}
}

func GetPostgresByIdent(ctx context.Context, id ident.Ident) (*Postgres, error) {
	teamSlug, environmentName, clusterName, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return GetPostgres(ctx, teamSlug, environmentName, clusterName)
}

func GetPostgres(ctx context.Context, teamSlug slug.Slug, environmentName string, clusterName string) (*Postgres, error) {
	return fromContext(ctx).postgresWatcher.Get(environmentName, teamSlug.String(), clusterName)
}

func ListForWorkload(ctx context.Context, workloadName string, teamSlug slug.Slug, environmentName string, references []nais_io_v1.CloudSqlInstance, orderBy *SQLInstanceOrder) (*SQLInstanceConnection, error) {
	all := fromContext(ctx).sqlInstanceWatcher.GetByNamespace(teamSlug.String(), watcher.InCluster(environmentName))

	ret := make([]*SQLInstance, 0)

	for _, ref := range references {
		name := workloadName
		if ref.Name != "" {
			name = ref.Name
		}
		for _, d := range all {
			if d.Obj.Name == name {
				ret = append(ret, d.Obj)
			}
		}
	}

	orderSQLInstances(ctx, ret, orderBy)

	return pagination.NewConnectionWithoutPagination(ret), nil
}

func orderSQLInstances(ctx context.Context, instances []*SQLInstance, orderBy *SQLInstanceOrder) {
	if orderBy == nil {
		orderBy = &SQLInstanceOrder{
			Field:     "NAME",
			Direction: model.OrderDirectionAsc,
		}
	}

	SortFilterSQLInstance.Sort(ctx, instances, orderBy.Field, orderBy.Direction)
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *SQLInstanceOrder) (*SQLInstanceConnection, error) {
	all := ListAllForTeam(ctx, teamSlug)
	orderSQLInstances(ctx, all, orderBy)

	instances := pagination.Slice(all, page)
	return pagination.NewConnection(instances, page, len(all)), nil
}

func ListAllForTeam(ctx context.Context, teamSlug slug.Slug) []*SQLInstance {
	all := fromContext(ctx).sqlInstanceWatcher.GetByNamespace(teamSlug.String())
	return watcher.Objects(all)
}

func ListSQLInstanceUsers(ctx context.Context, sqlInstance *SQLInstance, page *pagination.Pagination, orderBy *SQLInstanceUserOrder) (*SQLInstanceUserConnection, error) {
	adminUsers, err := fromContext(ctx).sqlAdminService.GetUsers(ctx, sqlInstance.ProjectID, sqlInstance.Name)
	if err != nil {
		var googleErr *googleapi.Error
		if errors.As(err, &googleErr) && googleErr.Code == 400 {
			// TODO: This was handled in the legacy code, keep it for now. Log?
			return pagination.EmptyConnection[*SQLInstanceUser](), nil
		}
		return nil, fmt.Errorf("getting SQL users")
	}

	all := make([]*SQLInstanceUser, len(adminUsers))
	for i, user := range adminUsers {
		all[i] = toSQLInstanceUser(user)
	}

	if orderBy == nil {
		orderBy = &SQLInstanceUserOrder{
			Field:     "NAME",
			Direction: model.OrderDirectionAsc,
		}
	}

	SortFilterSQLInstanceUser.Sort(ctx, all, orderBy.Field, orderBy.Direction)

	users := pagination.Slice(all, page)
	return pagination.NewConnection(users, page, len(all)), nil
}

func GetState(ctx context.Context, project, instance string) (SQLInstanceState, error) {
	i, err := fromContext(ctx).remoteSQLInstance.Load(ctx, instanceKey{projectID: project, name: instance})
	if err != nil {
		var googleErr *googleapi.Error
		if errors.As(err, &googleErr) && googleErr.Code == 404 {
			return SQLInstanceStateUnspecified, nil
		}
		return "", err
	}

	s := SQLInstanceState(i.State)
	if s == SQLInstanceStateRunnable && i.Settings != nil && i.Settings.ActivationPolicy == "NEVER" {
		return SQLInstanceStateStopped, nil
	}
	return s, nil
}

func MetricsFor(ctx context.Context, projectID, name string) (*SQLInstanceMetrics, error) {
	return &SQLInstanceMetrics{
		InstanceName: name,
		ProjectID:    projectID,
	}, nil
}

func CPUForInstance(ctx context.Context, projectID, instance string) (*SQLInstanceCPU, error) {
	return fromContext(ctx).sqlMetricsService.cpuForSQLInstance(ctx, projectID, instance)
}

func MemoryForInstance(ctx context.Context, projectID, instance string) (*SQLInstanceMemory, error) {
	return fromContext(ctx).sqlMetricsService.memoryForSQLInstance(ctx, projectID, instance)
}

func DiskForInstance(ctx context.Context, projectID, instance string) (*SQLInstanceDisk, error) {
	return fromContext(ctx).sqlMetricsService.diskForSQLInstance(ctx, projectID, instance)
}

func TeamSummaryCPU(ctx context.Context, projectID string) (*TeamServiceUtilizationSQLInstancesCPU, error) {
	return fromContext(ctx).sqlMetricsService.teamSummaryCPU(ctx, projectID)
}

func TeamSummaryMemory(ctx context.Context, projectID string) (*TeamServiceUtilizationSQLInstancesMemory, error) {
	return fromContext(ctx).sqlMetricsService.teamSummaryMemory(ctx, projectID)
}

func TeamSummaryDisk(ctx context.Context, projectID string) (*TeamServiceUtilizationSQLInstancesDisk, error) {
	return fromContext(ctx).sqlMetricsService.teamSummaryDisk(ctx, projectID)
}

func GrantPostgresAccess(ctx context.Context, input GrantPostgresAccessInput) error {
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
	annotations["euthanaisa.nais.io/kill-after"] = until.Format(time.RFC3339)

	labels := make(map[string]string)
	labels["euthanaisa.nais.io/enabled"] = "true"

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
		ResourceType:    activityLogEntryResourceTypePostgres,
		ResourceName:    input.ClusterName,
		EnvironmentName: ptr.To(input.EnvironmentName),
		TeamSlug:        ptr.To(input.TeamSlug),
		Data: PostgresGrantAccessActivityLogEntryData{
			Grantee: input.Grantee,
			Until:   until,
		},
	})
}

func createRoleBinding(ctx context.Context, input GrantPostgresAccessInput, name string, namespace string, annotations map[string]string, labels map[string]string) error {
	gvr := schema.GroupVersionResource{
		Group:    "rbac.authorization.k8s.io",
		Version:  "v1",
		Resource: "rolebindings",
	}
	client, err := fromContext(ctx).postgresWatcher.ImpersonatedClientWithNamespace(ctx, input.EnvironmentName, namespace, watcher.WithImpersonatedClientGVR(gvr))
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

func createRole(ctx context.Context, input GrantPostgresAccessInput, name string, namespace string, annotations map[string]string, labels map[string]string) error {
	gvr := schema.GroupVersionResource{
		Group:    "rbac.authorization.k8s.io",
		Version:  "v1",
		Resource: "roles",
	}
	client, err := fromContext(ctx).postgresWatcher.ImpersonatedClientWithNamespace(ctx, input.EnvironmentName, namespace, watcher.WithImpersonatedClientGVR(gvr))
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
	_, err := hasher.Write([]byte(fmt.Sprintf("%s-%s-%s", teamSlug.String(), grantee, name)))
	if err != nil {
		return "", err
	}
	hashStr := fmt.Sprintf("%08x", hasher.Sum32())
	return fmt.Sprintf("pg-grant-%s", hashStr), nil
}

package valkey

import (
	"context"
	"fmt"
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
	"github.com/nais/api/internal/thirdparty/aiven"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

func GetByIdent(ctx context.Context, id ident.Ident) (*Valkey, error) {
	teamSlug, environment, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return Get(ctx, teamSlug, environment, name)
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*Valkey, error) {
	prefix := instanceNamer(teamSlug, "")
	if !strings.HasPrefix(name, prefix) {
		name = instanceNamer(teamSlug, name)
	}
	return fromContext(ctx).client.watcher.Get(environment, teamSlug.String(), name)
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *ValkeyOrder) (*ValkeyConnection, error) {
	all := ListAllForTeam(ctx, teamSlug)
	orderValkey(ctx, all, orderBy)

	instances := pagination.Slice(all, page)
	return pagination.NewConnection(instances, page, len(all)), nil
}

func ListAllForTeam(ctx context.Context, teamSlug slug.Slug) []*Valkey {
	all := fromContext(ctx).client.watcher.GetByNamespace(teamSlug.String(), watcher.WithoutDeleted())
	return watcher.Objects(all)
}

func ListAccess(ctx context.Context, valkey *Valkey, page *pagination.Pagination, orderBy *ValkeyAccessOrder) (*ValkeyAccessConnection, error) {
	k8sClient := fromContext(ctx).client

	applicationAccess, err := k8sClient.getAccessForApplications(ctx, valkey.EnvironmentName, valkey.FullyQualifiedName(), valkey.TeamSlug)
	if err != nil {
		return nil, err
	}

	jobAccess, err := k8sClient.getAccessForJobs(ctx, valkey.EnvironmentName, valkey.FullyQualifiedName(), valkey.TeamSlug)
	if err != nil {
		return nil, err
	}

	all := make([]*ValkeyAccess, 0)
	all = append(all, applicationAccess...)
	all = append(all, jobAccess...)

	if orderBy == nil {
		orderBy = &ValkeyAccessOrder{
			Field:     "ACCESS",
			Direction: model.OrderDirectionAsc,
		}
	}
	SortFilterValkeyAccess.Sort(ctx, all, orderBy.Field, orderBy.Direction)

	ret := pagination.Slice(all, page)
	return pagination.NewConnection(ret, page, len(all)), nil
}

func ListForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName string, references []nais_io_v1.Valkey, orderBy *ValkeyOrder) (*ValkeyConnection, error) {
	all := fromContext(ctx).client.watcher.GetByNamespace(teamSlug.String(), watcher.InCluster(environmentName))
	ret := make([]*Valkey, 0)

	for _, ref := range references {
		for _, d := range all {
			if d.Obj.FullyQualifiedName() == instanceNamer(teamSlug, ref.Instance) {
				ret = append(ret, d.Obj)
			}
		}
	}

	orderValkey(ctx, ret, orderBy)
	return pagination.NewConnectionWithoutPagination(ret), nil
}

func orderValkey(ctx context.Context, instances []*Valkey, orderBy *ValkeyOrder) {
	if orderBy == nil {
		orderBy = &ValkeyOrder{
			Field:     "NAME",
			Direction: model.OrderDirectionAsc,
		}
	}

	SortFilterValkey.Sort(ctx, instances, orderBy.Field, orderBy.Direction)
}

func Create(ctx context.Context, input CreateValkeyInput) (*CreateValkeyPayload, error) {
	if err := input.Validate(ctx); err != nil {
		return nil, err
	}

	namespace := input.TeamSlug.String()
	client, err := fromContext(ctx).watcher.ImpersonatedClientWithNamespace(ctx, input.EnvironmentName, namespace)
	if err != nil {
		return nil, err
	}

	machine, err := machineTypeFromTierAndMemory(input.Tier, input.Memory)
	if err != nil {
		return nil, err
	}

	res := &unstructured.Unstructured{}
	res.SetAPIVersion("aiven.io/v1alpha1")
	res.SetKind("Valkey")
	res.SetName(instanceNamer(input.TeamSlug, input.Name))
	res.SetNamespace(namespace)
	res.SetAnnotations(kubernetes.WithCommonAnnotations(nil, authz.ActorFromContext(ctx).User.Identity()))
	kubernetes.SetManagedByConsoleLabel(res)

	aivenProject, err := aiven.GetProject(ctx, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	res.Object["spec"] = map[string]any{
		"cloudName":             "google-europe-north1",
		"plan":                  machine.AivenPlan,
		"project":               aivenProject.ID,
		"projectVpcId":          aivenProject.VPC,
		"terminationProtection": true,
		"tags": map[string]any{
			"environment": input.EnvironmentName,
			"team":        namespace,
			"tenant":      fromContext(ctx).tenantName,
		},
	}

	if input.MaxMemoryPolicy != nil {
		maxMemoryPolicy := input.MaxMemoryPolicy.ToAivenString()
		err := unstructured.SetNestedField(res.Object, maxMemoryPolicy, "spec", "userConfig", "valkey_maxmemory_policy")
		if err != nil {
			return nil, err
		}
	}

	ret, err := client.Create(ctx, res, metav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return nil, apierror.ErrAlreadyExists
		}
		return nil, err
	}

	err = aiven.UpsertPrometheusServiceIntegration(ctx, fromContext(ctx).watcher, ret, aivenProject, input.EnvironmentName)
	if err != nil {
		return nil, fmt.Errorf("creating Prometheus service integration: %w", err)
	}

	err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activitylog.ActivityLogEntryActionCreated,
		Actor:           authz.ActorFromContext(ctx).User,
		ResourceType:    ActivityLogEntryResourceTypeValkey,
		ResourceName:    input.Name,
		EnvironmentName: ptr.To(input.EnvironmentName),
		TeamSlug:        ptr.To(input.TeamSlug),
	})
	if err != nil {
		return nil, err
	}

	valkey, err := toValkey(ret, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	return &CreateValkeyPayload{
		Valkey: valkey,
	}, nil
}

func Update(ctx context.Context, input UpdateValkeyInput) (*UpdateValkeyPayload, error) {
	if err := input.Validate(ctx); err != nil {
		return nil, err
	}

	client, err := fromContext(ctx).watcher.ImpersonatedClientWithNamespace(ctx, input.EnvironmentName, input.TeamSlug.String())
	if err != nil {
		return nil, err
	}

	valkey, err := client.Get(ctx, instanceNamer(input.TeamSlug, input.Name), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !kubernetes.HasManagedByConsoleLabel(valkey) {
		return nil, apierror.Errorf("Valkey %s/%s is not managed by Console", input.TeamSlug, input.Name)
	}

	changes := make([]*ValkeyUpdatedActivityLogEntryDataUpdatedField, 0)

	res, err := updatePlan(valkey, input)
	if err != nil {
		return nil, err
	}
	changes = append(changes, res...)

	res, err = updateMaxMemoryPolicy(valkey, input)
	if err != nil {
		return nil, err
	}
	changes = append(changes, res...)

	if len(changes) == 0 {
		vk, err := toValkey(valkey, input.EnvironmentName)
		if err != nil {
			return nil, err
		}

		return &UpdateValkeyPayload{
			Valkey: vk,
		}, nil
	}

	valkey.SetAnnotations(kubernetes.WithCommonAnnotations(valkey.GetAnnotations(), authz.ActorFromContext(ctx).User.Identity()))

	ret, err := client.Update(ctx, valkey, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	aivenProject, err := aiven.GetProject(ctx, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	err = aiven.UpsertPrometheusServiceIntegration(ctx, fromContext(ctx).watcher, ret, aivenProject, input.EnvironmentName)
	if err != nil {
		return nil, fmt.Errorf("creating Prometheus service integration: %w", err)
	}

	err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activitylog.ActivityLogEntryActionUpdated,
		Actor:           authz.ActorFromContext(ctx).User,
		ResourceType:    ActivityLogEntryResourceTypeValkey,
		ResourceName:    input.Name,
		EnvironmentName: ptr.To(input.EnvironmentName),
		TeamSlug:        ptr.To(input.TeamSlug),
		Data: ValkeyUpdatedActivityLogEntryData{
			UpdatedFields: changes,
		},
	})
	if err != nil {
		return nil, err
	}

	valkeyUpdated, err := toValkey(ret, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	return &UpdateValkeyPayload{
		Valkey: valkeyUpdated,
	}, nil
}

func updatePlan(valkey *unstructured.Unstructured, input UpdateValkeyInput) ([]*ValkeyUpdatedActivityLogEntryDataUpdatedField, error) {
	changes := make([]*ValkeyUpdatedActivityLogEntryDataUpdatedField, 0)

	desired, err := machineTypeFromTierAndMemory(input.Tier, input.Memory)
	if err != nil {
		return nil, err
	}

	oldPlan, found, err := unstructured.NestedString(valkey.Object, "spec", "plan")
	if err != nil {
		return nil, err
	}
	if !found {
		// .spec.plan is a required field
		return nil, fmt.Errorf("missing .spec.plan in Valkey resource")
	}

	if oldPlan == desired.AivenPlan {
		return changes, nil
	}

	oldMachine, err := machineTypeFromPlan(oldPlan)
	if err != nil {
		return nil, err
	}

	if input.Tier != oldMachine.Tier {
		changes = append(changes, &ValkeyUpdatedActivityLogEntryDataUpdatedField{
			Field:    "tier",
			OldValue: ptr.To(oldMachine.Tier.String()),
			NewValue: ptr.To(input.Tier.String()),
		})
	}

	if input.Memory != oldMachine.Memory {
		changes = append(changes, &ValkeyUpdatedActivityLogEntryDataUpdatedField{
			Field:    "memory",
			OldValue: ptr.To(oldMachine.Memory.String()),
			NewValue: ptr.To(input.Memory.String()),
		})
	}

	if err := unstructured.SetNestedField(valkey.Object, desired.AivenPlan, "spec", "plan"); err != nil {
		return nil, err
	}

	return changes, nil
}

func updateMaxMemoryPolicy(valkey *unstructured.Unstructured, input UpdateValkeyInput) ([]*ValkeyUpdatedActivityLogEntryDataUpdatedField, error) {
	changes := make([]*ValkeyUpdatedActivityLogEntryDataUpdatedField, 0)

	if input.MaxMemoryPolicy == nil {
		return changes, nil
	}

	oldAivenPolicy, found, err := unstructured.NestedString(valkey.Object, "spec", "userConfig", "valkey_maxmemory_policy")
	if err != nil {
		return nil, err
	}

	if found && oldAivenPolicy == input.MaxMemoryPolicy.ToAivenString() {
		return changes, nil
	}
	// continue if not found so that we explicitly set the policy on the resource

	var oldValue *string
	if found {
		oldPolicy, err := ValkeyMaxMemoryPolicyFromAivenString(oldAivenPolicy)
		if err != nil {
			return nil, err
		}
		oldValue = ptr.To(oldPolicy.String())
	}

	changes = append(changes, &ValkeyUpdatedActivityLogEntryDataUpdatedField{
		Field:    "maxMemoryPolicy",
		OldValue: oldValue,
		NewValue: ptr.To(input.MaxMemoryPolicy.String()),
	})

	maxMemoryPolicy := input.MaxMemoryPolicy.ToAivenString()
	if err := unstructured.SetNestedField(valkey.Object, maxMemoryPolicy, "spec", "userConfig", "valkey_maxmemory_policy"); err != nil {
		return nil, err
	}

	return changes, nil
}

func Delete(ctx context.Context, input DeleteValkeyInput) (*DeleteValkeyPayload, error) {
	if err := input.Validate(ctx); err != nil {
		return nil, err
	}

	name := instanceNamer(input.TeamSlug, input.Name)
	client, err := fromContext(ctx).watcher.ImpersonatedClientWithNamespace(ctx, input.EnvironmentName, input.TeamSlug.String())
	if err != nil {
		return nil, err
	}

	valkey, err := client.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if !kubernetes.HasManagedByConsoleLabel(valkey) {
		return nil, apierror.Errorf("Valkey %s/%s is not managed by Console", input.TeamSlug, input.Name)
	}

	terminationProtection, found, err := unstructured.NestedBool(valkey.Object, "spec", "terminationProtection")
	if err != nil {
		return nil, err
	}
	if found && terminationProtection {
		if err := unstructured.SetNestedField(valkey.Object, false, "spec", "terminationProtection"); err != nil {
			return nil, err
		}

		_, err = client.Update(ctx, valkey, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("removing deletion protection: %w", err)
		}
	}

	if err := fromContext(ctx).watcher.Delete(ctx, input.EnvironmentName, input.TeamSlug.String(), name); err != nil {
		return nil, err
	}

	if err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activitylog.ActivityLogEntryActionDeleted,
		Actor:           authz.ActorFromContext(ctx).User,
		ResourceType:    ActivityLogEntryResourceTypeValkey,
		ResourceName:    input.Name,
		EnvironmentName: ptr.To(input.EnvironmentName),
		TeamSlug:        ptr.To(input.TeamSlug),
	}); err != nil {
		return nil, err
	}

	return &DeleteValkeyPayload{
		ValkeyDeleted: ptr.To(true),
	}, nil
}

func State(ctx context.Context, v *Valkey) (ValkeyState, error) {
	s, err := fromContext(ctx).aivenClient.ServiceGet(ctx, v.AivenProject, v.FullyQualifiedName())
	if err != nil {
		return ValkeyStateUnknown, err
	}

	switch s.State {
	case "RUNNING":
		return ValkeyStateRunning, nil
	case "REBALANCING":
		return ValkeyStateRebalancing, nil
	case "REBUILDING":
		return ValkeyStateRebuilding, nil
	case "POWEROFF":
		return ValkeyStatePoweroff, nil
	default:
		return ValkeyStateUnknown, nil
	}
}

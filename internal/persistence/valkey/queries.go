package valkey

import (
	"context"
	"errors"
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
	"github.com/nais/pgrator/pkg/api"
	naiscrd "github.com/nais/pgrator/pkg/api/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var (
	specTerminationProtection = []string{"spec", "terminationProtection"}
	specMaxMemoryPolicy       = []string{"spec", "userConfig", "valkey_maxmemory_policy"}
	specNotifyKeyspaceEvents  = []string{"spec", "userConfig", "valkey_notify_keyspace_events"}
)

func GetByIdent(ctx context.Context, id ident.Ident) (*Valkey, error) {
	teamSlug, environment, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return Get(ctx, teamSlug, environment, name)
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*Valkey, error) {
	v, err := fromContext(ctx).naisWatcher.Get(environment, teamSlug.String(), name)
	if errors.Is(err, &watcher.ErrorNotFound{}) {
		prefix := instanceNamer(teamSlug, "")
		if !strings.HasPrefix(name, prefix) {
			name = instanceNamer(teamSlug, name)
		}
		v, err = fromContext(ctx).watcher.Get(environment, teamSlug.String(), name)
	}
	return v, err
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *ValkeyOrder) (*ValkeyConnection, error) {
	all := ListAllForTeam(ctx, teamSlug)
	orderValkey(ctx, all, orderBy)

	instances := pagination.Slice(all, page)
	return pagination.NewConnection(instances, page, len(all)), nil
}

func ListAllForTeam(ctx context.Context, teamSlug slug.Slug) []*Valkey {
	all := fromContext(ctx).watcher.GetByNamespace(teamSlug.String(), watcher.WithoutDeleted())
	allNais := fromContext(ctx).naisWatcher.GetByNamespace(teamSlug.String(), watcher.WithoutDeleted())
	all = append(all, allNais...)
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
	all := fromContext(ctx).watcher.GetByNamespace(teamSlug.String(), watcher.InCluster(environmentName))
	allNais := fromContext(ctx).naisWatcher.GetByNamespace(teamSlug.String(), watcher.InCluster(environmentName))
	all = append(all, allNais...)
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

	client, err := newK8sClient(ctx, input.EnvironmentName, input.TeamSlug)
	if err != nil {
		return nil, err
	}

	// Ensure there's no existing Aiven Valkey with the same name
	// This can be removed when we manage all valkeys through Console
	_, err = fromContext(ctx).watcher.Get(input.EnvironmentName, input.TeamSlug.String(), instanceNamer(input.TeamSlug, input.Name))
	if err == nil {
		return nil, apierror.Errorf("Valkey with the name %q already exists, but are not yet managed through Console.", input.Name)
	} else if !errors.Is(err, &watcher.ErrorNotFound{}) {
		return nil, err
	}

	res := &naiscrd.Valkey{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Valkey",
			APIVersion: "nais.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      input.Name,
			Namespace: input.TeamSlug.String(),
		},
		Spec: naiscrd.ValkeySpec{
			Tier:   toMapperatorTier(input.Tier),
			Memory: toMapperatorMemory(input.Memory),
		},
	}
	res.SetAnnotations(kubernetes.WithCommonAnnotations(nil, authz.ActorFromContext(ctx).User.Identity()))
	kubernetes.SetManagedByConsoleLabel(res)

	if input.MaxMemoryPolicy != nil {
		res.Spec.MaxMemoryPolicy = naiscrd.ValkeyMaxMemoryPolicy(input.MaxMemoryPolicy.ToAivenString())
	}
	if input.NotifyKeyspaceEvents != nil {
		res.Spec.NotifyKeyspaceEvents = *input.NotifyKeyspaceEvents
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

	obj, err := kubernetes.ToUnstructured(res)
	if err != nil {
		return nil, err
	}

	if _, err = client.Create(ctx, obj, metav1.CreateOptions{}); err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return nil, apierror.ErrAlreadyExists
		}
		return nil, err
	}

	valkey, err := toValkeyFromNais(res, input.EnvironmentName)
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

	client, err := newK8sClient(ctx, input.EnvironmentName, input.TeamSlug)
	if err != nil {
		return nil, err
	}

	valkey, err := client.Get(ctx, input.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	concreteValkey, err := kubernetes.ToConcrete[naiscrd.Valkey](valkey)
	if err != nil {
		return nil, err
	}

	changes := make([]*ValkeyUpdatedActivityLogEntryDataUpdatedField, 0)
	updateFuncs := []func(*naiscrd.Valkey, UpdateValkeyInput) ([]*ValkeyUpdatedActivityLogEntryDataUpdatedField, error){
		updateTier,
		updateMemory,
		updateMaxMemoryPolicy,
		updateNotifyKeyspaceEvents,
	}

	for _, f := range updateFuncs {
		res, err := f(concreteValkey, input)
		if err != nil {
			return nil, err
		}
		changes = append(changes, res...)
	}

	if len(changes) == 0 {
		v, err := kubernetes.ToConcrete[naiscrd.Valkey](valkey)
		if err != nil {
			return nil, err
		}
		vk, err := toValkeyFromNais(v, input.EnvironmentName)
		if err != nil {
			return nil, err
		}

		return &UpdateValkeyPayload{
			Valkey: vk,
		}, nil
	}

	obj, err := kubernetes.ToUnstructured(concreteValkey)
	if err != nil {
		return nil, err
	}

	obj.SetAnnotations(kubernetes.WithCommonAnnotations(obj.GetAnnotations(), authz.ActorFromContext(ctx).User.Identity()))

	ret, err := client.Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
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

	retValkey, err := kubernetes.ToConcrete[naiscrd.Valkey](ret)
	if err != nil {
		return nil, err
	}

	valkeyUpdated, err := toValkeyFromNais(retValkey, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	return &UpdateValkeyPayload{
		Valkey: valkeyUpdated,
	}, nil
}

func updateTier(valkey *naiscrd.Valkey, input UpdateValkeyInput) ([]*ValkeyUpdatedActivityLogEntryDataUpdatedField, error) {
	changes := make([]*ValkeyUpdatedActivityLogEntryDataUpdatedField, 0)

	origTier := fromMapperatorTier(valkey.Spec.Tier)
	if input.Tier != origTier {
		changes = append(changes, &ValkeyUpdatedActivityLogEntryDataUpdatedField{
			Field:    "tier",
			OldValue: ptr.To(origTier.String()),
			NewValue: ptr.To(input.Tier.String()),
		})
	}

	valkey.Spec.Tier = toMapperatorTier(input.Tier)

	return changes, nil
}

func updateMemory(valkey *naiscrd.Valkey, input UpdateValkeyInput) ([]*ValkeyUpdatedActivityLogEntryDataUpdatedField, error) {
	changes := make([]*ValkeyUpdatedActivityLogEntryDataUpdatedField, 0)

	origMemory := fromMapperatorMemory(valkey.Spec.Memory)
	if input.Memory != origMemory {
		changes = append(changes, &ValkeyUpdatedActivityLogEntryDataUpdatedField{
			Field:    "memory",
			OldValue: ptr.To(origMemory.String()),
			NewValue: ptr.To(input.Memory.String()),
		})
	}

	valkey.Spec.Memory = toMapperatorMemory(input.Memory)

	return changes, nil
}

func updateMaxMemoryPolicy(valkey *naiscrd.Valkey, input UpdateValkeyInput) ([]*ValkeyUpdatedActivityLogEntryDataUpdatedField, error) {
	if input.MaxMemoryPolicy == nil {
		return nil, nil
	}

	if string(valkey.Spec.MaxMemoryPolicy) == input.MaxMemoryPolicy.ToAivenString() {
		return nil, nil
	}

	var oldMMP *string
	if valkey.Spec.MaxMemoryPolicy != "" {
		old, err := ValkeyMaxMemoryPolicyFromAivenString(valkey.Spec.MaxMemoryPolicy)
		if err != nil {
			return nil, fmt.Errorf("parsing existing max memory policy: %w", err)
		}
		oldMMP = ptr.To(old.String())
	}

	changes := make([]*ValkeyUpdatedActivityLogEntryDataUpdatedField, 0)

	changes = append(changes, &ValkeyUpdatedActivityLogEntryDataUpdatedField{
		Field:    "maxMemoryPolicy",
		OldValue: oldMMP,
		NewValue: ptr.To(input.MaxMemoryPolicy.String()),
	})

	valkey.Spec.MaxMemoryPolicy = naiscrd.ValkeyMaxMemoryPolicy(input.MaxMemoryPolicy.ToAivenString())

	return changes, nil
}

func updateNotifyKeyspaceEvents(valkey *naiscrd.Valkey, input UpdateValkeyInput) ([]*ValkeyUpdatedActivityLogEntryDataUpdatedField, error) {
	if input.NotifyKeyspaceEvents == nil {
		return nil, nil
	}

	if string(valkey.Spec.NotifyKeyspaceEvents) == *input.NotifyKeyspaceEvents {
		return nil, nil
	}

	changes := make([]*ValkeyUpdatedActivityLogEntryDataUpdatedField, 0)

	var oldValue *string
	if valkey.Spec.NotifyKeyspaceEvents != "" {
		oldValue = ptr.To(string(valkey.Spec.NotifyKeyspaceEvents))
	}
	changes = append(changes, &ValkeyUpdatedActivityLogEntryDataUpdatedField{
		Field:    "notifyKeyspaceEvents",
		OldValue: oldValue,
		NewValue: input.NotifyKeyspaceEvents,
	})

	valkey.Spec.NotifyKeyspaceEvents = *input.NotifyKeyspaceEvents
	return changes, nil
}

func Delete(ctx context.Context, input DeleteValkeyInput) (*DeleteValkeyPayload, error) {
	if err := input.Validate(ctx); err != nil {
		return nil, err
	}

	client, err := newK8sClient(ctx, input.EnvironmentName, input.TeamSlug)
	if err != nil {
		return nil, err
	}

	valkey, err := client.Get(ctx, input.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if !kubernetes.HasManagedByConsoleLabel(valkey) {
		return nil, apierror.Errorf("Valkey %s/%s is not managed by Console", input.TeamSlug, input.Name)
	}

	annotations := valkey.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if annotations[api.AllowDeletionAnnotation] != "true" {
		annotations[api.AllowDeletionAnnotation] = "true"
		valkey.SetAnnotations(annotations)

		_, err = client.Update(ctx, valkey, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("set allow deletion annotation: %w", err)
		}
	}

	if err := fromContext(ctx).naisWatcher.Delete(ctx, input.EnvironmentName, input.TeamSlug.String(), input.Name); err != nil {
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
		// The Valkey instance may not have been created in Aiven yet, or it has been deleted.
		// In both cases, we return "unknown" state rather than an error.
		if aiven.IsNotFound(err) {
			return ValkeyStateUnknown, nil
		}
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

func toMapperatorTier(tier ValkeyTier) naiscrd.ValkeyTier {
	switch tier {
	case ValkeyTierSingleNode:
		return naiscrd.ValkeyTierSingleNode
	case ValkeyTierHighAvailability:
		return naiscrd.ValkeyTierHighAvailability
	default:
		return ""
	}
}

func fromMapperatorTier(tier naiscrd.ValkeyTier) ValkeyTier {
	switch tier {
	case naiscrd.ValkeyTierSingleNode:
		return ValkeyTierSingleNode
	case naiscrd.ValkeyTierHighAvailability:
		return ValkeyTierHighAvailability
	default:
		return ""
	}
}

func toMapperatorMemory(memory ValkeyMemory) naiscrd.ValkeyMemory {
	switch memory {
	case ValkeyMemoryGB1:
		return naiscrd.ValkeyMemory1GB
	case ValkeyMemoryGB4:
		return naiscrd.ValkeyMemory4GB
	case ValkeyMemoryGB8:
		return naiscrd.ValkeyMemory8GB
	case ValkeyMemoryGB14:
		return naiscrd.ValkeyMemory14GB
	case ValkeyMemoryGB28:
		return naiscrd.ValkeyMemory28GB
	case ValkeyMemoryGB56:
		return naiscrd.ValkeyMemory56GB
	case ValkeyMemoryGB112:
		return naiscrd.ValkeyMemory112GB
	case ValkeyMemoryGB200:
		return naiscrd.ValkeyMemory200GB
	default:
		return ""
	}
}

func fromMapperatorMemory(memory naiscrd.ValkeyMemory) ValkeyMemory {
	switch memory {
	case naiscrd.ValkeyMemory1GB:
		return ValkeyMemoryGB1
	case naiscrd.ValkeyMemory4GB:
		return ValkeyMemoryGB4
	case naiscrd.ValkeyMemory8GB:
		return ValkeyMemoryGB8
	case naiscrd.ValkeyMemory14GB:
		return ValkeyMemoryGB14
	case naiscrd.ValkeyMemory28GB:
		return ValkeyMemoryGB28
	case naiscrd.ValkeyMemory56GB:
		return ValkeyMemoryGB56
	case naiscrd.ValkeyMemory112GB:
		return ValkeyMemoryGB112
	case naiscrd.ValkeyMemory200GB:
		return ValkeyMemoryGB200
	default:
		return ""
	}
}

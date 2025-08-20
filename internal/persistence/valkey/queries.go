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
	return fromContext(ctx).client.watcher.Get(environment, teamSlug.String(), name)
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *ValkeyOrder) (*ValkeyConnection, error) {
	all := ListAllForTeam(ctx, teamSlug)
	orderValkey(ctx, all, orderBy)

	instances := pagination.Slice(all, page)
	return pagination.NewConnection(instances, page, len(all)), nil
}

func ListAllForTeam(ctx context.Context, teamSlug slug.Slug) []*Valkey {
	all := fromContext(ctx).client.watcher.GetByNamespace(teamSlug.String())
	return watcher.Objects(all)
}

func ListAccess(ctx context.Context, valkey *Valkey, page *pagination.Pagination, orderBy *ValkeyAccessOrder) (*ValkeyAccessConnection, error) {
	k8sClient := fromContext(ctx).client

	applicationAccess, err := k8sClient.getAccessForApplications(ctx, valkey.EnvironmentName, valkey.Name, valkey.TeamSlug)
	if err != nil {
		return nil, err
	}

	jobAccess, err := k8sClient.getAccessForJobs(ctx, valkey.EnvironmentName, valkey.Name, valkey.TeamSlug)
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
			if d.Obj.Name == valkeyNamer(teamSlug, ref.Instance) {
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

	client, err := fromContext(ctx).watcher.ImpersonatedClient(ctx, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	plan, err := planFromTierAndSize(input.Tier, input.Size)
	if err != nil {
		return nil, err
	}

	res := &unstructured.Unstructured{}
	res.SetAPIVersion("aiven.io/v1alpha1")
	res.SetKind("Valkey")
	res.SetName(valkeyNamer(input.TeamSlug, input.Name))
	res.SetNamespace(input.TeamSlug.String())
	res.SetAnnotations(kubernetes.WithCommonAnnotations(nil, authz.ActorFromContext(ctx).User.Identity()))
	kubernetes.SetManagedByConsoleLabel(res)

	aivenProject, err := aiven.GetProject(ctx, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	res.Object["spec"] = map[string]any{
		"cloudName":             "google-europe-north1",
		"plan":                  plan,
		"project":               aivenProject.ID,
		"projectVpcId":          aivenProject.VPC,
		"terminationProtection": true,
		"tags": map[string]any{
			"environment": input.EnvironmentName,
			"team":        input.TeamSlug.String(),
			"tenant":      fromContext(ctx).tenantName,
		},
	}

	if input.MaxMemoryPolicy != nil {
		maxMemoryPolicy := strings.ReplaceAll(strings.ToLower(input.MaxMemoryPolicy.String()), "_", "-")
		err := unstructured.SetNestedField(res.Object, maxMemoryPolicy, "spec", "userConfig", "valkey_maxmemory_policy")
		if err != nil {
			return nil, err
		}
	}

	ret, err := client.Namespace(input.TeamSlug.String()).Create(ctx, res, metav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return nil, apierror.ErrAlreadyExists
		}
		return nil, err
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

	client, err := fromContext(ctx).watcher.ImpersonatedClient(ctx, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	valkey, err := client.Namespace(input.TeamSlug.String()).Get(ctx, valkeyNamer(input.TeamSlug, input.Name), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !kubernetes.HasManagedByConsoleLabel(valkey) {
		return nil, apierror.Errorf("Valkey %s/%s is not managed by Console", input.TeamSlug, input.Name)
	}

	changes := []*ValkeyUpdatedActivityLogEntryDataUpdatedField{}

	plan, err := planFromTierAndSize(input.Tier, input.Size)
	if err != nil {
		return nil, err
	}

	oldPlan, found, err := unstructured.NestedString(valkey.Object, "spec", "plan")
	if err != nil {
		return nil, err
	}

	if !found || oldPlan != plan {
		tier, size, err := tierAndSizeFromPlan(oldPlan)
		if err != nil {
			return nil, err
		}

		if input.Tier != tier {
			changes = append(changes, &ValkeyUpdatedActivityLogEntryDataUpdatedField{
				Field: "tier",
				OldValue: func() *string {
					if found {
						return ptr.To(tier.String())
					}
					return nil
				}(),
				NewValue: ptr.To(input.Tier.String()),
			})
		}
		if input.Size != size {
			changes = append(changes, &ValkeyUpdatedActivityLogEntryDataUpdatedField{
				Field: "size",
				OldValue: func() *string {
					if found {
						return ptr.To(size.String())
					}
					return nil
				}(),
				NewValue: ptr.To(input.Size.String()),
			})
		}

		err = unstructured.SetNestedField(valkey.Object, plan, "spec", "plan")
		if err != nil {
			return nil, err
		}
	}

	if input.MaxMemoryPolicy != nil {
		oldMMP, found, err := unstructured.NestedString(valkey.Object, "spec", "userConfig", "valkey_maxmemory_policy")
		if err != nil {
			return nil, err
		}

		if !found || oldMMP != input.MaxMemoryPolicy.String() {
			changes = append(changes, &ValkeyUpdatedActivityLogEntryDataUpdatedField{
				Field: "maxMemoryPolicy",
				OldValue: func() *string {
					if found {
						return ptr.To(strings.ReplaceAll(strings.ToUpper(oldMMP), "-", "_"))
					}
					return nil
				}(),
				NewValue: ptr.To(input.MaxMemoryPolicy.String()),
			})

			maxMemoryPolicy := strings.ReplaceAll(strings.ToLower(input.MaxMemoryPolicy.String()), "_", "-")
			err := unstructured.SetNestedField(valkey.Object, maxMemoryPolicy, "spec", "userConfig", "valkey_maxmemory_policy")
			if err != nil {
				return nil, err
			}
		}
	}

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

	ret, err := client.Namespace(input.TeamSlug.String()).Update(ctx, valkey, metav1.UpdateOptions{})
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

	valkeyUpdated, err := toValkey(ret, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	return &UpdateValkeyPayload{
		Valkey: valkeyUpdated,
	}, nil
}

var aivenPlans = map[string]ValkeyTier{
	"business": ValkeyTierHighAvailability,
	"startup":  ValkeyTierSingleNode,
}

var aivenSizes = map[string]ValkeySize{
	"1":   ValkeySizeRAM1gb,
	"4":   ValkeySizeRAM4gb,
	"8":   ValkeySizeRAM8gb,
	"14":  ValkeySizeRAM14gb,
	"28":  ValkeySizeRAM28gb,
	"56":  ValkeySizeRAM56gb,
	"112": ValkeySizeRAM112gb,
	"200": ValkeySizeRAM200gb,
}

func planFromTierAndSize(tier ValkeyTier, size ValkeySize) (string, error) {
	plan := ""

	if size == ValkeySizeRAM1gb && tier == ValkeyTierSingleNode {
		return "", apierror.Errorf("invalid Valkey size for tier %s: %s",
			tier, size)
	}

	for name, planTier := range aivenPlans {
		if planTier == tier {
			plan += name + "-"
			break
		}
	}
	if plan == "" {
		return "", apierror.Errorf("invalid Valkey tier: %s", tier)
	}

	planSize := ""
	for aivenSize, sz := range aivenSizes {
		if sz == size {
			planSize = aivenSize
			break
		}
	}
	if planSize == "" {
		return "", apierror.Errorf("invalid Valkey size: %s", size)
	}
	plan += planSize

	return plan, nil
}

func tierAndSizeFromPlan(plan string) (ValkeyTier, ValkeySize, error) {
	t, s, ok := strings.Cut(plan, "-")
	if !ok {
		return "", "", fmt.Errorf("invalid Valkey plan: %s", plan)
	}

	tier, ok := aivenPlans[t]
	if !ok {
		return "", "", fmt.Errorf("invalid Valkey tier: %s", t)
	}

	size, ok := aivenSizes[s]
	if !ok {
		return "", "", fmt.Errorf("invalid Valkey size: %s", s)
	}
	if size == ValkeySizeRAM1gb && tier == ValkeyTierSingleNode {
		return "", "", fmt.Errorf("invalid Valkey size for tier %s: %s", tier, s)
	}

	return tier, size, nil
}

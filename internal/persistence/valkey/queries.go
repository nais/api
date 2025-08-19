package valkey

import (
	"context"
	"strings"

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
	client, err := fromContext(ctx).watcher.ImpersonatedClient(ctx, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	plan, err := aivenPlan(input.Tier, input.Size)
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

	valkey, err := toValkey(ret, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	return &CreateValkeyPayload{
		Valkey: valkey,
	}, nil
}

func Update(ctx context.Context, input UpdateValkeyInput) (*UpdateValkeyPayload, error) {
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

	plan, err := aivenPlan(input.Tier, input.Size)
	if err != nil {
		return nil, err
	}

	err = unstructured.SetNestedField(valkey.Object, plan, "spec", "plan")
	if err != nil {
		return nil, err
	}

	if input.MaxMemoryPolicy != nil {
		maxMemoryPolicy := strings.ReplaceAll(strings.ToLower(input.MaxMemoryPolicy.String()), "_", "-")
		err := unstructured.SetNestedField(valkey.Object, maxMemoryPolicy, "spec", "userConfig", "valkey_maxmemory_policy")
		if err != nil {
			return nil, err
		}
	}

	ret, err := client.Namespace(input.TeamSlug.String()).Update(ctx, valkey, metav1.UpdateOptions{})
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

func aivenPlan(tier ValkeyTier, size ValkeySize) (string, error) {
	plan := ""

	switch tier {
	case ValkeyTierHighAvailability:
		plan = "business-"
	case ValkeyTierSingleNode:
		plan = "startup-"
	default:
		return "", apierror.Errorf("invalid Valkey tier: %s", tier)
	}

	switch size {
	case ValkeySizeRAM1gb:
		if tier == ValkeyTierHighAvailability {
			return "", apierror.Errorf("invalid Valkey size for tier %s: %s", tier, size)
		}
		plan += "1"
	case ValkeySizeRAM4gb:
		plan += "4"
	case ValkeySizeRAM8gb:
		plan += "8"
	case ValkeySizeRAM14gb:
		plan += "14"
	case ValkeySizeRAM28gb:
		plan += "28"
	case ValkeySizeRAM56gb:
		plan += "56"
	case ValkeySizeRAM112gb:
		plan += "112"
	case ValkeySizeRAM200gb:
		plan += "200"
	default:
		return "", apierror.Errorf("invalid Valkey size: %s", size)
	}

	return plan, nil
}

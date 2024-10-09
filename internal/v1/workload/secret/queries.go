package secret

import (
	"context"
	"encoding/base64"
	"slices"
	"strings"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination) (*SecretConnection, error) {
	all := fromContext(ctx).secretWatcher.GetByNamespace(teamSlug.String())

	jobs := pagination.Slice(watcher.Objects(all), page)
	return pagination.NewConnection(jobs, page, int32(len(all))), nil
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*Secret, error) {
	return fromContext(ctx).secretWatcher.Get(environment, teamSlug.String(), name)
}

func GetByIdent(ctx context.Context, id ident.Ident) (*Secret, error) {
	teamSlug, env, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, teamSlug, env, name)
}

func GetSecretData(ctx context.Context, teamSlug slug.Slug, environmentName, name string) ([]*SecretVariable, error) {
	client, err := fromContext(ctx).secretWatcher.ImpersonatedClient(ctx, environmentName)
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

	vars := make([]*SecretVariable, 0, len(data))
	for k, v := range data {
		val, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, err
		}

		vars = append(vars, &SecretVariable{
			Name:  k,
			Value: string(val),
		})
	}

	slices.SortFunc(vars, func(a, b *SecretVariable) int {
		return strings.Compare(a.Name, b.Name)
	})

	return vars, nil
}

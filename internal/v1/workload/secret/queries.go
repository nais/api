package secret

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
)

var ErrSecretUnmanaged = errors.New("secret is not managed by console")

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

func Create(ctx context.Context, teamSlug slug.Slug, environment, name string, data []*SecretVariableInput) (*Secret, error) {
	client, err := fromContext(ctx).secretWatcher.ImpersonatedClient(ctx, environment)
	if err != nil {
		return nil, err
	}

	nameErrs := validation.IsDNS1123Subdomain(name)
	if len(nameErrs) > 0 {
		return nil, fmt.Errorf("invalid name %q: %s", name, strings.Join(nameErrs, ", "))
	}

	err = validateSecretData(data)
	if err != nil {
		return nil, err
	}

	actor := authz.ActorFromContext(ctx)

	secret := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:        name,
			Namespace:   teamSlug.String(),
			Annotations: annotations(actor.User.Identity()),
			Labels:      labels(),
		},
		Type: corev1.SecretTypeOpaque,
		Data: secretTupleToMap(data),
	}

	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(secret)
	if err != nil {
		return nil, err
	}

	s, err := client.Namespace(teamSlug.String()).Create(ctx, &unstructured.Unstructured{Object: u}, v1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("%q: %w", name, ErrSecretUnmanaged)
		}
		return nil, fmt.Errorf("creating secret: %w", err)
	}

	return toGraphSecret(s, environment), nil
}

func annotations(user string) map[string]string {
	return map[string]string{
		secretAnnotationLastModifiedBy: user,
		secretAnnotationLastModifiedAt: time.Now().Format(time.RFC3339),
		"reloader.stakater.com/match":  "true",
	}
}

func labels() map[string]string {
	return map[string]string{
		secretLabelManagedByKey: secretLabelManagedByVal,
	}
}

func validateSecretData(data []*SecretVariableInput) error {
	seen := make(map[string]bool)

	for _, d := range data {
		_, found := seen[d.Name]
		if found {
			return fmt.Errorf("duplicate key: %q", d.Name)
		}

		seen[d.Name] = true

		if len(d.Name) > validation.DNS1123SubdomainMaxLength {
			return fmt.Errorf("%q is too long: %d characters, max %d", d.Name, len(d.Name), validation.DNS1123SubdomainMaxLength)
		}

		isEnvVarName := validation.IsEnvVarName(d.Name)
		if len(isEnvVarName) > 0 {
			return fmt.Errorf("%q: %s", d.Name, strings.Join(isEnvVarName, ", "))
		}
	}

	return nil
}

func secretTupleToMap(data []*SecretVariableInput) map[string][]byte {
	ret := make(map[string][]byte, len(data))
	for _, tuple := range data {
		ret[tuple.Name] = []byte(tuple.Value)
	}
	return ret
}

package k8s

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/nais/api/internal/slug"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	consoleSecretLabelKey = "nais.io/managed-by"
	consoleSecretLabelVal = "console"
)

// TODO: implement impersonation
//  authorization: check that requesting user has access to the requested team?

func (c *Client) Secrets(ctx context.Context, team slug.Slug) ([]*model.EnvSecret, error) {
	ret := make([]*model.EnvSecret, 0)
	namespace := team.String()

	for name, infs := range c.informers {
		secrets := make([]model.Secret, 0)

		objs, err := infs.SecretInformer.Lister().Secrets(namespace).List(labels.Everything())
		if err != nil {
			return nil, c.error(ctx, err, "listing secrets")
		}

		for _, obj := range objs {
			secrets = append(secrets, *toGraphSecret(name, obj))
		}
		ret = append(ret, toGraphEnvSecret(name, team, secrets...))
	}

	slices.SortFunc(ret, func(a, b *model.EnvSecret) int {
		return cmp.Compare(a.Env.Name, b.Env.Name)
	})

	return ret, nil
}

func (c *Client) Secret(ctx context.Context, name string, team slug.Slug, env string) (*model.Secret, error) {
	namespace := team.String()
	infs, ok := c.informers[env]
	if !ok {
		return nil, fmt.Errorf("no informer for env %q", env)
	}

	secret, err := infs.SecretInformer.Lister().Secrets(namespace).Get(name)
	if err != nil {
		return nil, c.error(ctx, err, "getting secret")
	}

	return toGraphSecret(env, secret), nil
}

func (c *Client) CreateSecret(ctx context.Context, name string, team slug.Slug, env string, data []*model.SecretTupleInput) (*model.Secret, error) {
	namespace := team.String()
	cli, ok := c.clientSets[env]
	if !ok {
		return nil, fmt.Errorf("no clientset for env %q", env)
	}

	created, err := cli.CoreV1().Secrets(namespace).Create(ctx, kubeSecret(name, namespace, data), metav1.CreateOptions{})
	if err != nil {
		return nil, c.error(ctx, err, "creating secret")
	}

	return toGraphSecret(env, created), nil
}

func (c *Client) UpdateSecret(ctx context.Context, name string, team slug.Slug, env string, data []*model.SecretTupleInput) (*model.Secret, error) {
	namespace := team.String()
	cli, ok := c.clientSets[env]
	if !ok {
		return nil, fmt.Errorf("no clientset for env %q", env)
	}
	updated, err := cli.CoreV1().Secrets(namespace).Update(ctx, kubeSecret(name, namespace, data), metav1.UpdateOptions{})
	if err != nil {
		return nil, c.error(ctx, err, "updating secret")
	}
	return toGraphSecret(env, updated), nil
}

func (c *Client) DeleteSecret(ctx context.Context, name string, team slug.Slug, env string) (bool, error) {
	namespace := team.String()
	cli, ok := c.clientSets[env]
	if !ok {
		return false, fmt.Errorf("no clientset for env %q", env)
	}

	err := cli.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return false, c.error(ctx, err, "deleting secret")
	}

	return true, nil
}

func kubeSecret(name, namespace string, data []*model.SecretTupleInput) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				consoleSecretLabelKey: consoleSecretLabelVal,
			},
		},
		Data: secretTupleToMap(data),
	}
}

func toGraphEnvSecret(env string, team slug.Slug, secret ...model.Secret) *model.EnvSecret {
	return &model.EnvSecret{
		Env:     model.Env{Team: team.String(), Name: env},
		Secrets: secret,
	}
}

func toGraphSecret(env string, obj *corev1.Secret) *model.Secret {
	return &model.Secret{
		ID:   makeSecretIdent(env, obj.GetNamespace(), obj.GetName()),
		Name: obj.Name,
		Data: secretBytesToString(obj.Data),
	}
}

func secretBytesToString(data map[string][]byte) map[string]string {
	ret := make(map[string]string, len(data))
	for key, value := range data {
		ret[key] = string(value)
	}
	return ret
}

func secretTupleToMap(data []*model.SecretTupleInput) map[string][]byte {
	ret := make(map[string][]byte, len(data))
	for _, tuple := range data {
		ret[tuple.Key] = []byte(tuple.Value)
	}
	return ret
}

func makeSecretIdent(env, namespace, name string) scalar.Ident {
	return scalar.SecretIdent("secret_" + env + "_" + namespace + "_" + name)
}

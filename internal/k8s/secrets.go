package k8s

import (
	"context"
	"slices"

	"github.com/nais/api/internal/slug"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	consoleSecretLabelKey = "nais.io/managed-by"
	consoleSecretLabelVal = "console"
)

// TODO: implement impersonation
func (c *Client) Secrets(ctx context.Context, team string) ([]*model.EnvSecret, error) {
	ret := make([]*model.EnvSecret, 0)

	for name, infs := range c.informers {
		secrets := make([]model.Secret, 0)
		objs, err := infs.SecretInformer.Lister().Secrets(team).List(labels.Everything())
		if err != nil {
			return nil, c.error(ctx, err, "listing secrets")
		}
		for _, obj := range objs {
			secrets = append(secrets, *toGraphSecret(name, obj))
		}
		ret = append(ret, toGraphEnvSecret(name, team, secrets...))
	}

	slices.SortFunc(ret, func(a, b *model.EnvSecret) int {
		if a.Env.Name < b.Env.Name {
			return -1
		}
		if a.Env.Name > b.Env.Name {
			return 1
		}
		return 0
	})

	return ret, nil
}

func (c *Client) Secret(ctx context.Context, name, team, env string) (*model.Secret, error) {
	// TODO: clientSet might be nil of no matching env
	secret, err := c.informers[env].SecretInformer.Lister().Secrets(team).Get(name)
	if err != nil {
		return nil, c.error(ctx, err, "getting secret")
	}

	return toGraphSecret(env, secret), nil
}

func (c *Client) CreateSecret(ctx context.Context, secret *model.Secret) (*model.Secret, error) {
	env := "foo"
	namespace := secret.GQLVars.Team.String()
	// TODO: clientSet might be nil of no matching env
	created, err := c.clientSets[env].CoreV1().Secrets(namespace).Create(ctx, toKubeSecret(secret), metav1.CreateOptions{})
	if err != nil {
		return nil, c.error(ctx, err, "creating secret")
	}
	return toGraphSecret(env, created), nil
}

func (c *Client) UpdateSecret(ctx context.Context, secret *model.Secret) (*model.Secret, error) {
	env := "foo"
	namespace := secret.GQLVars.Team.String()
	// TODO: clientSet might be nil of no matching env
	updated, err := c.clientSets[env].CoreV1().Secrets(namespace).Update(ctx, toKubeSecret(secret), metav1.UpdateOptions{})
	if err != nil {
		return nil, c.error(ctx, err, "updating secret")
	}
	return toGraphSecret(env, updated), nil
}

func (c *Client) DeleteSecret(ctx context.Context, name string, team slug.Slug, env string) error {
	namespace := team.String()
	err := c.clientSets[env].CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return c.error(ctx, err, "deleting secret")
	}
	return nil
}

func toKubeSecret(secret *model.Secret) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.Name,
			Namespace: secret.GQLVars.Team.String(),
			Labels: map[string]string{
				consoleSecretLabelKey: consoleSecretLabelVal,
			},
		},
		StringData: secret.Data,
	}
}

func toGraphEnvSecret(env string, team string, secret ...model.Secret) *model.EnvSecret {
	return &model.EnvSecret{
		Env:     model.Env{Team: team, Name: env},
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

func makeSecretIdent(env, namespace, name string) scalar.Ident {
	return scalar.SecretIdent("secret_" + env + "_" + namespace + "_" + name)
}

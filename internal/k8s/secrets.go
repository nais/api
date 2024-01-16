package k8s

import (
	"context"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

const (
	consoleSecretLabelKey = "nais.io/managed-by"
	consoleSecretLabelVal = "console"
)

// TODO: implement impersonation
func (c *Client) Secrets(ctx context.Context, team string) ([]*model.Secret, error) {
	ret := make([]*model.Secret, 0)

	secretReq, err := labels.NewRequirement(consoleSecretLabelKey, selection.Equals, []string{consoleSecretLabelVal})
	if err != nil {
		return nil, c.error(ctx, err, "creating label selector")
	}
	secretSelector := labels.NewSelector().Add(*secretReq)

	for env, infs := range c.informers {
		objs, err := infs.SecretInformer.Lister().Secrets(team).List(secretSelector)
		if err != nil {
			return nil, c.error(ctx, err, "listing applications")
		}

		for _, obj := range objs {
			ret = append(ret, toGraphSecret(obj, env))
		}
	}
	return ret, nil
}

func (c *Client) Secret(ctx context.Context, name, team, env string) (*model.Secret, error) {
	secret, err := c.informers[env].SecretInformer.Lister().Secrets(team).Get(name)
	if err != nil {
		return nil, c.error(ctx, err, "getting secret")
	}

	return toGraphSecret(secret, env), nil
}

func (c *Client) CreateSecret(ctx context.Context, secret *model.Secret) (*model.Secret, error) {
	env := secret.Env.Name
	namespace := secret.GQLVars.Team.String()
	created, err := c.clientSets[env].CoreV1().Secrets(namespace).Create(ctx, toKubeSecret(secret), metav1.CreateOptions{})
	if err != nil {
		return nil, c.error(ctx, err, "creating secret")
	}
	return toGraphSecret(env, created), nil
}

func (c *Client) UpdateSecret(ctx context.Context, secret *model.Secret) (*model.Secret, error) {
	env := secret.Env.Name
	namespace := secret.GQLVars.Team.String()
	updated, err := c.clientSets[env].CoreV1().Secrets(namespace).Update(ctx, toKubeSecret(secret), metav1.UpdateOptions{})
	if err != nil {
		return nil, c.error(ctx, err, "updating secret")
	}
	return toGraphSecret(env, updated), nil
}

func (c *Client) DeleteSecret(ctx context.Context, secret *model.Secret) error {
	env := secret.Env.Name
	namespace := secret.GQLVars.Team.String()
	err := c.clientSets[env].CoreV1().Secrets(namespace).Delete(ctx, secret.Name, metav1.DeleteOptions{})
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

func toGraphSecret(obj *corev1.Secret, env string) *model.Secret {
	return &model.Secret{
		ID: makeSecretIdent(env, obj.GetNamespace(), obj.GetName()),
		Env: model.Env{
			Name: env,
		},
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

package k8s

import (
	"cmp"
	"context"
	"fmt"
	naisv1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"slices"

	"github.com/nais/api/internal/slug"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	consoleSecretLabelKey = "nais.io/managed-by"
	consoleSecretLabelVal = "console"
)

// TODO: implement impersonation
//  authorization: check that requesting user has access to the requested team?

func (c *Client) Secrets(ctx context.Context, team slug.Slug) ([]*model.EnvSecret, error) {
	impersonatedClients, err := c.impersonationClientCreator(ctx)
	if err != nil {
		return nil, c.error(ctx, err, "impersonation")
	}
	ret := make([]*model.EnvSecret, 0)
	namespace := team.String()

	for env, clientSet := range impersonatedClients {
		secrets := make([]model.Secret, 0)
		kubeSecrets, err := clientSet.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})

		if err != nil {
			return nil, c.error(ctx, err, "listing secrets")
		}

		objs, err := c.informers[env].AppInformer.Lister().ByNamespace(team.String()).List(labels.Everything())
		if err != nil {
			return nil, c.error(ctx, err, fmt.Sprintf("getting application %s.%s", env, team))
		}

		// we want a map: Secret -> [App]
		appsForSecrets := make(map[string][]string)
		for _ ,obj := range objs {
		    u := obj.(*unstructured.Unstructured)
			app := &naisv1alpha1.Application{}

			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, app); err != nil {
				return nil, fmt.Errorf("converting to application: %w", err)
			}

 			for _, secret := range app.Spec.EnvFrom {

				as, ok := appsForSecrets[secret.Secret]
				if !ok {
					appsForSecrets[secret.Secret] = []string{app.Name}
				} else  {
					appsForSecrets[secret.Secret] = append(as, app.Name)
				}
			}

			for _, secret := range app.Spec.FilesFrom {
				as, ok := appsForSecrets[secret.Secret]
				if !ok {
					appsForSecrets[secret.Secret] = []string{app.Name}
				} else  {
					appsForSecrets[secret.Secret] = append(as, app.Name)
				}
			}
		}

		for _, obj := range kubeSecrets.Items {
			as, ok := appsForSecrets[obj.Name]
			if !ok {
				as = make([]string, 0)
			}
			secrets = append(secrets, *toGraphSecret(env, &obj, as))
		}
		ret = append(ret, toGraphEnvSecret(env, team, secrets...))
	}

	slices.SortFunc(ret, func(a, b *model.EnvSecret) int {
		return cmp.Compare(a.Env.Name, b.Env.Name)
	})

	return ret, nil
}

func (c *Client) Secret(ctx context.Context, name string, team slug.Slug, env string) (*model.Secret, error) {

	impersonatedClients, err := c.impersonationClientCreator(ctx)
	if err != nil {
		return nil, c.error(ctx, err, "impersonation")
	}

	namespace := team.String()
	cli, ok := impersonatedClients[env]
	if !ok {
		return nil, fmt.Errorf("no informer for env %q", env)
	}

	secret, err := cli.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})

	if err != nil {
		return nil, c.error(ctx, err, "getting secret")
	}

	return toGraphSecret(env, secret, make([]string, 0)), nil
}

func (c *Client) CreateSecret(ctx context.Context, name string, team slug.Slug, env string, data []*model.SecretTupleInput) (*model.Secret, error) {

	impersonatedClients, err := c.impersonationClientCreator(ctx)
	if err != nil {
		return nil, c.error(ctx, err, "impersonation")
	}

	namespace := team.String()
	cli, ok := impersonatedClients[env]
	if !ok {
		return nil, fmt.Errorf("no clientset for env %q", env)
	}

	created, err := cli.CoreV1().Secrets(namespace).Create(ctx, kubeSecret(name, namespace, data), metav1.CreateOptions{})
	if err != nil {
		return nil, c.error(ctx, err, "creating secret")
	}

	return toGraphSecret(env, created, make([]string,0)), nil
}

func (c *Client) UpdateSecret(ctx context.Context, name string, team slug.Slug, env string, data []*model.SecretTupleInput) (*model.Secret, error) {

	impersonatedClients, err := c.impersonationClientCreator(ctx)
	if err != nil {
		return nil, c.error(ctx, err, "impersonation")
	}

	namespace := team.String()
	cli, ok := impersonatedClients[env]
	if !ok {
		return nil, fmt.Errorf("no clientset for env %q", env)
	}
	updated, err := cli.CoreV1().Secrets(namespace).Update(ctx, kubeSecret(name, namespace, data), metav1.UpdateOptions{})
	if err != nil {
		return nil, c.error(ctx, err, "updating secret")
	}
	return toGraphSecret(env, updated, make([]string, 0)), nil
}

func (c *Client) DeleteSecret(ctx context.Context, name string, team slug.Slug, env string) (bool, error) {

	impersonatedClients, err := c.impersonationClientCreator(ctx)
	if err != nil {
		return false, c.error(ctx, err, "impersonation")
	}

	namespace := team.String()
	cli, ok := impersonatedClients[env]
	if !ok {
		return false, fmt.Errorf("no clientset for env %q", env)
	}

	err = cli.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
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

// toGraphSecret accepts apps as an empty list for cases where only the secret is getting
// updated
func toGraphSecret(
	env string,
	obj *corev1.Secret,
	apps []string,
) *model.Secret {
	apps = slices.Compact(apps)
	return &model.Secret{
		ID:   makeSecretIdent(env, obj.GetNamespace(), obj.GetName()),
		Name: obj.Name,
		Data: secretBytesToString(obj.Data),
		Apps: apps,
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

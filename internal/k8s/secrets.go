package k8s

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	naisv1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

const (
	secretLabelKey = "nais.io/managed-by"
	secretLabelVal = "console"
)

// Secrets lists all secrets for a given team in all environments
func (c *Client) Secrets(ctx context.Context, team slug.Slug) ([]*model.EnvSecret, error) {
	envSecrets := make([]*model.EnvSecret, 0)

	impersonatedClients, err := c.impersonationClientCreator(ctx)
	if err != nil {
		return nil, c.error(ctx, err, "impersonation")
	}

	for env, clientSet := range impersonatedClients {
		namespace := team.String()
		opts := metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", secretLabelKey, secretLabelVal),
		}

		kubeSecrets, err := clientSet.CoreV1().Secrets(namespace).List(ctx, opts)
		if err != nil {
			return nil, c.error(ctx, err, "listing secrets")
		}

		appsForSecrets, err := c.mapAppsBySecret(ctx, team, env)
		if err != nil {
			return nil, c.error(ctx, err, "mapping apps to secrets")
		}

		graphSecrets := make([]model.Secret, 0)
		for _, secret := range kubeSecrets.Items {
			if !secretIsManagedByConsole(secret) {
				continue
			}

			apps, ok := appsForSecrets[secret.Name]
			if !ok {
				apps = make([]string, 0)
			}

			graphSecrets = append(graphSecrets, *toGraphSecret(env, &secret, apps))
		}
		envSecrets = append(envSecrets, toGraphEnvSecret(env, team, graphSecrets...))
	}

	slices.SortFunc(envSecrets, func(a, b *model.EnvSecret) int {
		return cmp.Compare(a.Env.Name, b.Env.Name)
	})

	return envSecrets, nil
}

func (c *Client) Secret(ctx context.Context, name string, team slug.Slug, env string) (*model.Secret, error) {
	impersonatedClients, err := c.impersonationClientCreator(ctx)
	if err != nil {
		return nil, c.error(ctx, err, "impersonation")
	}

	clientSet, ok := impersonatedClients[env]
	if !ok {
		return nil, fmt.Errorf("no client set for env %q", env)
	}

	namespace := team.String()
	opts := metav1.GetOptions{}
	secret, err := clientSet.CoreV1().Secrets(namespace).Get(ctx, name, opts)
	if err != nil {
		return nil, c.error(ctx, err, "listing secrets")
	}

	appsForSecrets, err := c.mapAppsBySecret(ctx, team, env)
	if err != nil {
		return nil, c.error(ctx, err, "mapping apps to secrets")
	}

	if !secretIsManagedByConsole(*secret) {
		return nil, fmt.Errorf("secret %q is not managed by console", secret.GetName())
	}

	apps, ok := appsForSecrets[secret.Name]
	if !ok {
		apps = make([]string, 0)
	}

	return toGraphSecret(env, secret, apps), nil

}

func (c *Client) CreateSecret(ctx context.Context, name string, team slug.Slug, env string, data []*model.SecretTupleInput) (*model.Secret, error) {
	impersonatedClients, err := c.impersonationClientCreator(ctx)
	if err != nil {
		return nil, c.error(ctx, err, "impersonation")
	}

	nameErrs := validation.IsDNS1123Subdomain(name)
	if len(nameErrs) > 0 {
		return nil, fmt.Errorf("invalid name %q: %s", name, strings.Join(nameErrs, ", "))
	}

	err = validateSecretData(data)
	if err != nil {
		return nil, err
	}

	namespace := team.String()
	cli, ok := impersonatedClients[env]
	if !ok {
		return nil, fmt.Errorf("no client set for env %q", env)
	}

	user := authz.ActorFromContext(ctx).User.Identity()
	secret := kubeSecret(name, namespace, user, data)
	created, err := cli.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return nil, c.error(ctx, err, "creating secret")
	}

	return toGraphSecret(env, created, make([]string, 0)), nil
}

func (c *Client) UpdateSecret(ctx context.Context, name string, team slug.Slug, env string, data []*model.SecretTupleInput) (*model.Secret, error) {
	impersonatedClients, err := c.impersonationClientCreator(ctx)
	if err != nil {
		return nil, c.error(ctx, err, "impersonation")
	}

	err = validateSecretData(data)
	if err != nil {
		return nil, err
	}

	namespace := team.String()
	cli, ok := impersonatedClients[env]
	if !ok {
		return nil, fmt.Errorf("no client set for env %q", env)
	}

	existing, err := cli.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, c.error(ctx, err, "getting existing secret")
	}

	if !secretIsManagedByConsole(*existing) {
		return nil, fmt.Errorf("secret %q is not managed by console", existing.GetName())
	}

	user := authz.ActorFromContext(ctx).User.Identity()
	secret := kubeSecret(name, namespace, user, data)
	updated, err := cli.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
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

	existing, err := cli.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, c.error(ctx, err, "getting existing secret")
	}

	if !secretIsManagedByConsole(*existing) {
		return false, fmt.Errorf("secret %q is not managed by console", existing.GetName())
	}

	err = cli.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return false, c.error(ctx, err, "deleting secret")
	}

	return true, nil
}

// mapAppsBySecret returns a map of secrets to a list of apps that references said secret
func (c *Client) mapAppsBySecret(ctx context.Context, team slug.Slug, env string) (map[string][]string, error) {
	// fetch apps to build map of apps that use each secret
	apps, err := c.informers[env].AppInformer.Lister().ByNamespace(team.String()).List(labels.Everything())
	if err != nil {
		return nil, c.error(ctx, err, fmt.Sprintf("listing applications for %q in %q", team, env))
	}

	// we want a map: Secret -> [App]
	appsBySecret := make(map[string][]string)
	for _, obj := range apps {
		u := obj.(*unstructured.Unstructured)
		app := &naisv1alpha1.Application{}

		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, app); err != nil {
			return nil, fmt.Errorf("converting to application: %w", err)
		}

		for _, secret := range app.Spec.EnvFrom {
			as, ok := appsBySecret[secret.Secret]
			if !ok {
				appsBySecret[secret.Secret] = []string{app.Name}
			} else {
				appsBySecret[secret.Secret] = append(as, app.Name)
			}
		}

		for _, secret := range app.Spec.FilesFrom {
			as, ok := appsBySecret[secret.Secret]
			if !ok {
				appsBySecret[secret.Secret] = []string{app.Name}
			} else {
				appsBySecret[secret.Secret] = append(as, app.Name)
			}
		}
	}

	return appsBySecret, nil
}

func secretIsManagedByConsole(secret corev1.Secret) bool {
	secretLabel, ok := secret.GetLabels()[secretLabelKey]
	hasConsoleLabel := ok && secretLabel == secretLabelVal

	isOpaque := secret.Type == corev1.SecretTypeOpaque || secret.Type == "kubernetes.io/Opaque"
	hasOwnerReferences := len(secret.GetOwnerReferences()) > 0
	hasFinalizers := len(secret.GetFinalizers()) > 0

	typeLabel, ok := secret.GetLabels()["type"]
	isJwker := ok && typeLabel == "jwker.nais.io"

	return hasConsoleLabel && isOpaque && !hasOwnerReferences && !hasFinalizers && !isJwker
}

func kubeSecret(name, namespace, user string, data []*model.SecretTupleInput) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"console.nais.io/last-modified-by": user,
				"console.nais.io/last-modified-at": time.Now().Format(time.RFC3339),
			},
			Labels: map[string]string{
				secretLabelKey: secretLabelVal,
			},
		},
		Data: secretTupleToMap(data),
		Type: corev1.SecretTypeOpaque,
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
func toGraphSecret(env string, obj *corev1.Secret, apps []string) *model.Secret {
	// sort first as Compact only removes consecutive duplicates
	slices.Sort(apps)
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

func validateSecretData(data []*model.SecretTupleInput) error {
	seen := make(map[string]bool)

	for _, d := range data {
		_, found := seen[d.Key]
		if found {
			return fmt.Errorf("duplicate key: %q", d.Key)
		}

		seen[d.Key] = true

		if errs := validation.IsConfigMapKey(d.Key); len(errs) > 0 {
			return fmt.Errorf("invalid key: %q: %s", d.Key, strings.Join(errs, ", "))
		}
	}

	return nil
}

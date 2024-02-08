package k8s

import (
	"cmp"
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
)

const (
	secretLabelManagedByKey        = "nais.io/managed-by"
	secretLabelManagedByVal        = "console"
	secretAnnotationLastModifiedAt = "console.nais.io/last-modified-at"
	secretAnnotationLastModifiedBy = "console.nais.io/last-modified-by"
)

// Secrets lists all secrets for a given team in all environments
func (c *Client) Secrets(ctx context.Context, team slug.Slug) ([]*model.EnvSecret, error) {
	envSecrets := make([]*model.EnvSecret, 0)

	impersonatedClients, err := c.impersonationClientCreator(ctx)
	if err != nil {
		return nil, c.error(ctx, err, "impersonation")
	}

	for env, clientSet := range impersonatedClients {
		graphSecrets, err := c.listSecrets(ctx, team, env, clientSet)
		if err != nil {
			return nil, err
		}

		envSecrets = append(envSecrets, toGraphEnvSecret(env, team, graphSecrets...))
	}

	slices.SortFunc(envSecrets, func(a, b *model.EnvSecret) int {
		return cmp.Compare(a.Env.Name, b.Env.Name)
	})

	return envSecrets, nil
}

// SecretsForEnv lists all secrets for a given team in a specific environment
func (c *Client) SecretsForEnv(ctx context.Context, team slug.Slug, env string) ([]*model.Secret, error) {
	impersonatedClients, err := c.impersonationClientCreator(ctx)
	if err != nil {
		return nil, c.error(ctx, err, "impersonation")
	}

	clientSet, ok := impersonatedClients[env]
	if !ok {
		return nil, fmt.Errorf("no client set for env %q", env)
	}

	return c.listSecrets(ctx, team, env, clientSet)
}

func (c *Client) listSecrets(ctx context.Context, team slug.Slug, env string, clientSet kubernetes.Interface) ([]*model.Secret, error) {
	namespace := team.String()
	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", secretLabelManagedByKey, secretLabelManagedByVal),
	}

	kubeSecrets, err := clientSet.CoreV1().Secrets(namespace).List(ctx, opts)
	if err != nil {
		return nil, c.error(ctx, err, "listing secrets")
	}

	appsForSecrets, err := c.mapAppsBySecret(ctx, team, env)
	if err != nil {
		return nil, c.error(ctx, err, "mapping apps to secrets")
	}

	graphSecrets := make([]*model.Secret, 0)
	for _, secret := range kubeSecrets.Items {
		if !secretIsManagedByConsole(secret) {
			continue
		}

		graphSecrets = append(graphSecrets, toGraphSecret(env, &secret, appsForSecrets[secret.Name]))
	}

	return graphSecrets, nil
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
		return nil, c.error(ctx, err, "getting secret")
	}

	if !secretIsManagedByConsole(*secret) {
		return nil, fmt.Errorf("secret %q is not managed by console", secret.GetName())
	}

	appsForSecrets, err := c.mapAppsBySecret(ctx, team, env)
	if err != nil {
		return nil, c.error(ctx, err, "mapping apps to secrets")
	}

	return toGraphSecret(env, secret, appsForSecrets[secret.Name]), nil
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

	cli, ok := impersonatedClients[env]
	if !ok {
		return nil, fmt.Errorf("no client set for env %q", env)
	}

	actor := authz.ActorFromContext(ctx)
	secret := kubeSecret(name, team, actor, data)
	namespace := team.String()
	created, err := cli.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return nil, c.error(ctx, err, "creating secret")
	}

	return toGraphSecret(env, created, nil), nil
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

	actor := authz.ActorFromContext(ctx)
	secret := kubeSecret(name, team, actor, data)
	updated, err := cli.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return nil, c.error(ctx, err, "updating secret")
	}

	return toGraphSecret(env, updated, nil), nil
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
func (c *Client) mapAppsBySecret(ctx context.Context, team slug.Slug, env string) (map[string][]*model.App, error) {
	// fetch apps to build map of apps that use each secret
	apps, err := c.informers[env].AppInformer.Lister().ByNamespace(team.String()).List(labels.Everything())
	if err != nil {
		return nil, c.error(ctx, err, fmt.Sprintf("listing applications for %q in %q", team, env))
	}

	// we want a map: Secret -> [App]
	appsBySecret := make(map[string][]*model.App)
	for _, obj := range apps {
		u := obj.(*unstructured.Unstructured)
		app, err := c.App(ctx, u.GetName(), team.String(), env)
		if err != nil {
			return nil, err
		}

		for _, secret := range app.GQLVars.Secrets {
			appsBySecret[secret] = append(appsBySecret[secret], app)
		}
	}

	return appsBySecret, nil
}

func secretIsManagedByConsole(secret corev1.Secret) bool {
	secretLabel, ok := secret.GetLabels()[secretLabelManagedByKey]
	hasConsoleLabel := ok && secretLabel == secretLabelManagedByVal

	isOpaque := secret.Type == corev1.SecretTypeOpaque || secret.Type == "kubernetes.io/Opaque"
	hasOwnerReferences := len(secret.GetOwnerReferences()) > 0
	hasFinalizers := len(secret.GetFinalizers()) > 0

	typeLabel, ok := secret.GetLabels()["type"]
	isJwker := ok && typeLabel == "jwker.nais.io"

	return hasConsoleLabel && isOpaque && !hasOwnerReferences && !hasFinalizers && !isJwker
}

func kubeSecret(name string, team slug.Slug, actor *authz.Actor, data []*model.SecretTupleInput) *corev1.Secret {
	namespace := team.String()
	user := actor.User.Identity()

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				secretAnnotationLastModifiedBy: user,
				secretAnnotationLastModifiedAt: time.Now().Format(time.RFC3339),
				"reloader.stakater.com/match":  "true",
			},
			Labels: map[string]string{
				secretLabelManagedByKey: secretLabelManagedByVal,
			},
		},
		Data: secretTupleToMap(data),
		Type: corev1.SecretTypeOpaque,
	}
}

func toGraphEnvSecret(env string, team slug.Slug, secrets ...*model.Secret) *model.EnvSecret {
	return &model.EnvSecret{
		Env:     model.Env{Team: team.String(), Name: env},
		Secrets: secrets,
	}
}

// toGraphSecret accepts apps as an empty list for cases where only the secret is getting
// updated
func toGraphSecret(env string, obj *corev1.Secret, apps []*model.App) *model.Secret {
	if apps == nil {
		apps = make([]*model.App, 0)
	}

	// sort first as Compact only removes consecutive duplicates
	slices.SortFunc(apps, func(a, b *model.App) int {
		return cmp.Compare(a.Name, b.Name)
	})

	apps = slices.Compact(apps)

	secret := &model.Secret{
		ID:   makeSecretIdent(env, obj.GetNamespace(), obj.GetName()),
		Name: obj.Name,
		Data: secretBytesToString(obj.Data),
		Apps: apps,
	}

	annotations := obj.GetAnnotations()
	modifiedBy, ok := annotations[secretAnnotationLastModifiedBy]
	if ok {
		secret.GQLVars.LastModifiedBy = modifiedBy
	}

	modifiedAt := annotations[secretAnnotationLastModifiedAt]
	modifiedAtTime, err := time.Parse(time.RFC3339, modifiedAt)
	if err == nil {
		secret.LastModifiedAt = &modifiedAtTime
	}

	return secret
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
		ret[tuple.Name] = []byte(tuple.Value)
	}
	return ret
}

func makeSecretIdent(env, namespace, name string) scalar.Ident {
	return scalar.SecretIdent("secret_" + env + "_" + namespace + "_" + name)
}

const envVarNameFmtErrMsg = "must consist of alphabetic characters, digits, '_', and must not start with a digit"

var envVarNameRegexp = regexp.MustCompile("^[_a-zA-Z][_a-zA-Z0-9]*$")

func validateSecretData(data []*model.SecretTupleInput) error {
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

		if !envVarNameRegexp.MatchString(d.Name) {
			return fmt.Errorf("%q is invalid: %s", d.Name, envVarNameFmtErrMsg)
		}
	}

	return nil
}

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
func (c *Client) Secrets(ctx context.Context, team slug.Slug) ([]*model.Secret, error) {
	secrets := make([]*model.Secret, 0)

	impersonatedClients, err := c.impersonationClientCreator(ctx)
	if err != nil {
		return nil, c.error(ctx, err, "impersonation")
	}

	for env, clientSet := range impersonatedClients {
		envSecrets, err := c.listSecrets(ctx, team, env, clientSet)
		if err != nil {
			return nil, err
		}

		secrets = slices.Concat(secrets, envSecrets)
	}

	return secrets, nil
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

	graphSecrets := make([]*model.Secret, 0)
	for _, secret := range kubeSecrets.Items {
		if !secretIsManagedByConsole(secret) {
			continue
		}

		graphSecrets = append(graphSecrets, toGraphSecret(env, team, &secret))
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

	return toGraphSecret(env, team, secret), nil
}

func (c *Client) AppsUsingSecret(ctx context.Context, obj *model.Secret) ([]*model.App, error) {
	apps, err := c.Apps(ctx, obj.GQLVars.Team.String())
	if err != nil {
		return nil, fmt.Errorf("fetching apps: %w", err)
	}

	matches := make([]*model.App, 0)

	for _, app := range apps {
		if app.Env.Name != obj.GQLVars.Env {
			continue
		}

		for _, secret := range app.GQLVars.Secrets {
			if secret == obj.Name {
				matches = append(matches, app)
				break
			}
		}
	}

	// sort first as Compact only removes consecutive duplicates
	slices.SortFunc(matches, func(a, b *model.App) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return slices.Compact(matches), nil
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

	return toGraphSecret(env, team, created), nil
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

	return toGraphSecret(env, team, updated), nil
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

func toGraphSecret(env string, team slug.Slug, obj *corev1.Secret) *model.Secret {
	secret := &model.Secret{
		ID:   makeSecretIdent(env, obj.GetNamespace(), obj.GetName()),
		Name: obj.Name,
		Data: secretBytesToString(obj.Data),
		GQLVars: model.SecretGQLVars{
			Env:  env,
			Team: team,
		},
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

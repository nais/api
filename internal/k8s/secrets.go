package k8s

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	secretLabelManagedByKey        = "nais.io/managed-by"
	secretLabelManagedByVal        = "console"
	secretAnnotationLastModifiedAt = "console.nais.io/last-modified-at"
	secretAnnotationLastModifiedBy = "console.nais.io/last-modified-by"
)

var ErrSecretUnmanaged = errors.New("secret is not managed by console")

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

func (c *Client) SecretsForApp(ctx context.Context, obj *model.App) ([]*model.Secret, error) {
	secrets, err := c.SecretsForEnv(ctx, obj.GQLVars.Team, obj.Env.Name)
	if err != nil {
		return nil, err
	}

	ret := make([]*model.Secret, 0)
	for _, secret := range secrets {
		if slices.Contains(obj.GQLVars.SecretNames, secret.Name) {
			ret = append(ret, secret)
		}
	}

	return ret, nil
}

func (c *Client) SecretsForNaisJob(ctx context.Context, obj *model.NaisJob) ([]*model.Secret, error) {
	secrets, err := c.SecretsForEnv(ctx, obj.GQLVars.Team, obj.Env.Name)
	if err != nil {
		return nil, err
	}

	ret := make([]*model.Secret, 0)
	for _, secret := range secrets {
		if slices.Contains(obj.GQLVars.SecretNames, secret.Name) {
			ret = append(ret, secret)
		}
	}

	return ret, nil
}

func (c *Client) listSecrets(ctx context.Context, team slug.Slug, env string, clientSet clients) ([]*model.Secret, error) {
	namespace := team.String()
	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", secretLabelManagedByKey, secretLabelManagedByVal),
	}

	kubeSecrets, err := clientSet.client.CoreV1().Secrets(namespace).List(ctx, opts)
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
	secret, err := clientSet.client.CoreV1().Secrets(namespace).Get(ctx, name, opts)
	if err != nil {
		return nil, c.error(ctx, err, "getting secret")
	}

	if !secretIsManagedByConsole(*secret) {
		return nil, fmt.Errorf("%q: %w", secret.GetName(), ErrSecretUnmanaged)
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

		for _, secret := range app.GQLVars.SecretNames {
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

func (c *Client) NaisJobsUsingSecret(ctx context.Context, obj *model.Secret) ([]*model.NaisJob, error) {
	naisjobs, err := c.NaisJobs(ctx, obj.GQLVars.Team.String())
	if err != nil {
		return nil, fmt.Errorf("fetching naisjobs: %w", err)
	}

	matches := make([]*model.NaisJob, 0)

	for _, job := range naisjobs {
		if job.Env.Name != obj.GQLVars.Env {
			continue
		}

		for _, secret := range job.GQLVars.SecretNames {
			if secret == obj.Name {
				matches = append(matches, job)
				break
			}
		}
	}

	// sort first as Compact only removes consecutive duplicates
	slices.SortFunc(matches, func(a, b *model.NaisJob) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return slices.Compact(matches), nil
}

func (c *Client) CreateSecret(ctx context.Context, name string, team slug.Slug, env string, data []*model.VariableInput) (*model.Secret, error) {
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
	created, err := cli.client.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("%q: %w", name, ErrSecretUnmanaged)
		}
		return nil, c.error(ctx, err, "creating secret")
	}

	return toGraphSecret(env, team, created), nil
}

func (c *Client) UpdateSecret(ctx context.Context, name string, team slug.Slug, env string, data []*model.VariableInput) (*model.Secret, error) {
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

	existing, err := cli.client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, c.error(ctx, err, "getting existing secret")
	}

	if !secretIsManagedByConsole(*existing) {
		return nil, fmt.Errorf("%q: %w", existing.GetName(), ErrSecretUnmanaged)
	}

	actor := authz.ActorFromContext(ctx)
	secret := kubeSecret(name, team, actor, data)
	updated, err := cli.client.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
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

	existing, err := cli.client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, c.error(ctx, err, "getting existing secret")
	}

	if !secretIsManagedByConsole(*existing) {
		return false, fmt.Errorf("%q: %w", existing.GetName(), ErrSecretUnmanaged)
	}

	err = cli.client.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return false, c.error(ctx, err, "deleting secret")
	}

	return true, nil
}

func secretIsManagedByConsole(secret corev1.Secret) bool {
	labels := secret.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	secretLabel, ok := labels[secretLabelManagedByKey]
	hasConsoleLabel := ok && secretLabel == secretLabelManagedByVal

	isOpaque := secret.Type == corev1.SecretTypeOpaque || secret.Type == "kubernetes.io/Opaque"
	hasOwnerReferences := len(secret.GetOwnerReferences()) > 0
	hasFinalizers := len(secret.GetFinalizers()) > 0

	typeLabel, ok := labels["type"]
	isJwker := ok && typeLabel == "jwker.nais.io"

	return hasConsoleLabel && isOpaque && !hasOwnerReferences && !hasFinalizers && !isJwker
}

func kubeSecret(name string, team slug.Slug, actor *authz.Actor, data []*model.VariableInput) *corev1.Secret {
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

func toGraphSecret(env string, teamSlug slug.Slug, obj *corev1.Secret) *model.Secret {
	secret := &model.Secret{
		ID:   scalar.SecretIdent(env, teamSlug, obj.GetName()),
		Name: obj.Name,
		Data: secretBytesToString(obj.Data),
		GQLVars: model.SecretGQLVars{
			Env:  env,
			Team: teamSlug,
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

func secretTupleToMap(data []*model.VariableInput) map[string][]byte {
	ret := make(map[string][]byte, len(data))
	for _, tuple := range data {
		ret[tuple.Name] = []byte(tuple.Value)
	}
	return ret
}

func validateSecretData(data []*model.VariableInput) error {
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
